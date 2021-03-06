// Copyright 2016-2017 VMware, Inc. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//    http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package network

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"strings"
	"sync"
	"syscall"

	log "github.com/Sirupsen/logrus"
	"github.com/docker/go-connections/nat"

	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/vim25/types"
	"github.com/vmware/vic/lib/config/executor"
	"github.com/vmware/vic/lib/portlayer/constants"
	"github.com/vmware/vic/lib/portlayer/exec"
	"github.com/vmware/vic/lib/spec"
	"github.com/vmware/vic/pkg/ip"
	"github.com/vmware/vic/pkg/kvstore"
	"github.com/vmware/vic/pkg/trace"
	"github.com/vmware/vic/pkg/uid"
)

const (
	pciSlotNumberBegin int32 = 0xc0
	pciSlotNumberEnd   int32 = 1 << 10
	pciSlotNumberInc   int32 = 1 << 5

	DefaultBridgeName = "bridge"
)

// Context denotes a networking context that represents a set of scopes, endpoints,
// and containers. Each context has its own separate IPAM.
type Context struct {
	sync.Mutex

	config *Configuration

	defaultBridgePool *AddressSpace
	defaultBridgeMask net.IPMask

	scopes       map[string]*Scope
	containers   map[string]*Container
	aliases      map[string][]*Container
	defaultScope *Scope

	kv kvstore.KeyValueStore
}

type AddContainerOptions struct {
	Scope   string
	IP      net.IP
	Aliases []string
	Ports   []string
}

func NewContext(config *Configuration, kv kvstore.KeyValueStore) (*Context, error) {
	defer trace.End(trace.Begin(""))

	if config == nil {
		return nil, fmt.Errorf("missing config")
	}

	bridgeRange := config.BridgeIPRange
	if bridgeRange == nil || len(bridgeRange.IP) == 0 || bridgeRange.IP.IsUnspecified() {
		var err error
		_, bridgeRange, err = net.ParseCIDR(constants.DefaultBridgeRange)
		if err != nil {
			return nil, err
		}
	}

	bridgeWidth := config.BridgeNetworkWidth
	if bridgeWidth == nil || len(*bridgeWidth) == 0 {
		w := net.CIDRMask(16, 32)
		bridgeWidth = &w
	}

	pones, pbits := bridgeRange.Mask.Size()
	mones, mbits := bridgeWidth.Size()
	if pbits != mbits || mones < pones {
		return nil, fmt.Errorf("bridge mask is not compatible with bridge pool mask")
	}

	ctx := &Context{
		config:            config,
		defaultBridgeMask: *bridgeWidth,
		defaultBridgePool: NewAddressSpaceFromNetwork(bridgeRange),
		scopes:            make(map[string]*Scope),
		containers:        make(map[string]*Container),
		aliases:           make(map[string][]*Container),
		kv:                kv,
	}

	n := ctx.config.ContainerNetworks[ctx.config.BridgeNetwork]
	if n == nil {
		return nil, fmt.Errorf("default bridge network %s not present in config", ctx.config.BridgeNetwork)
	}

	scopeData := &ScopeData{
		ScopeType: n.Type,
		Name:      n.Name,
	}
	s, err := ctx.newScope(scopeData)
	if err != nil {
		return nil, err
	}
	s.builtin = true
	ctx.defaultScope = s

	// add any bridge/external networks
	for nn, n := range ctx.config.ContainerNetworks {
		if nn == ctx.config.BridgeNetwork {
			continue // already added above
		}

		pools := make([]string, len(n.Pools))
		for i, p := range n.Pools {
			pools[i] = p.String()
		}

		subnet := net.IPNet{IP: n.Gateway.IP.Mask(n.Gateway.Mask), Mask: n.Gateway.Mask}

		scopeData = &ScopeData{
			ScopeType:  n.Type,
			Name:       nn,
			Subnet:     &subnet,
			Gateway:    n.Gateway.IP,
			DNS:        n.Nameservers,
			TrustLevel: n.TrustLevel,
			Pools:      pools,
		}

		s, err := ctx.newScope(scopeData)
		if err != nil {
			return nil, err
		}

		s.builtin = true
	}

	// load saved scopes in the kv store
	if kv != nil {
		values, err := kv.List(`context\.scopes\..+`)
		if err != nil && err != kvstore.ErrKeyNotFound {
			log.Warnf("error listing scopes from key value store: %s", err)
		} else {
			for k, v := range values {
				s := newScope(uid.NilUID, "", nil, &ScopeData{})
				if err := s.UnmarshalJSON(v); err != nil {
					log.Warnf("error loading scope data from key %s, skipping: %s", k, err)
					continue
				}

				var nn string
				switch s.Type() {
				case constants.BridgeScopeType:
					nn = "bridge"
				case constants.ExternalScopeType:
					nn = s.name
				}

				pg := config.PortGroups[nn]
				if pg == nil {
					log.Warnf("skipping adding scope %s: port group %s not found", s.name, nn)
					continue
				}

				s.network = pg

				if err := ctx.addScope(s); err != nil {
					log.Warnf("skipping adding scope %s: %s", s.name, err)
				}
			}
		}
	}

	return ctx, nil
}

func reserveGateway(gateway net.IP, subnet *net.IPNet, spaces []*AddressSpace) (net.IP, error) {
	defer trace.End(trace.Begin(""))
	if ip.IsUnspecifiedSubnet(subnet) {
		return nil, fmt.Errorf("cannot reserve gateway for nil subnet")
	}

	if !ip.IsUnspecifiedIP(gateway) {
		// verify gateway is routable address
		if !ip.IsRoutableIP(gateway, subnet) {
			return nil, fmt.Errorf("gateway address %s is not routable on network %s", gateway, subnet)
		}

		// optionally reserve it in one of the pools
		for _, p := range spaces {
			if err := p.ReserveIP4(gateway); err == nil {
				break
			}
		}

		return gateway, nil
	}

	// gateway is not specified, pick one from the available pools
	if len(spaces) > 0 {
		var err error
		if gateway, err = spaces[0].ReserveNextIP4(); err != nil {
			return nil, err
		}

		if !ip.IsRoutableIP(gateway, subnet) {
			return nil, fmt.Errorf("gateway address %s is not routable on network %s", gateway, subnet)
		}

		return gateway, nil
	}

	return nil, fmt.Errorf("could not reserve gateway address for network %s", subnet)
}

func (c *Context) addScope(s *Scope) error {
	defer trace.End(trace.Begin(""))

	if _, ok := c.scopes[s.name]; ok {
		return DuplicateResourceError{}
	}

	var err error
	var defaultPool bool
	var allzeros, allones net.IP
	var space *AddressSpace
	spaces := s.spaces
	subnet := s.subnet
	gateway := s.gateway

	// cleanup
	defer func() {
		if err == nil || space == nil || !defaultPool {
			return
		}

		for _, p := range spaces {
			// release DNS IPs
			for _, d := range s.dns {
				p.ReleaseIP4(d)
			}

			// release gateway
			if !ip.IsUnspecifiedIP(gateway) {
				p.ReleaseIP4(gateway)
			}

			// release all-ones and all-zeros addresses
			if !ip.IsUnspecifiedIP(allzeros) {
				p.ReleaseIP4(allzeros)
			}
			if !ip.IsUnspecifiedIP(allones) {
				p.ReleaseIP4(allones)
			}
		}

		c.defaultBridgePool.ReleaseIP4Range(space)
	}()

	// subnet may not be specified, e.g. for "external" networks
	if !ip.IsUnspecifiedSubnet(subnet) {
		// allocate the subnet
		space, defaultPool, err = c.reserveSubnet(subnet)
		if err != nil {
			return err
		}

		subnet = space.Network

		spaces, err = reservePools(space, spaces)
		if err != nil {
			return err
		}

		// reserve all-ones and all-zeros addresses, which are not routable and so
		// should not be handed out
		allones = ip.AllOnesAddr(subnet)
		allzeros = ip.AllZerosAddr(subnet)
		for _, p := range spaces {
			p.ReserveIP4(allones)
			p.ReserveIP4(allzeros)

			// reserve DNS IPs
			for _, d := range s.dns {
				if d.Equal(gateway) {
					continue // gateway will be reserved later
				}

				p.ReserveIP4(d)
			}
		}

		if gateway, err = reserveGateway(gateway, subnet, spaces); err != nil {
			return err
		}

		s.gateway = gateway
		s.spaces = spaces
		s.subnet = subnet
	}

	c.scopes[s.name] = s

	return nil
}

func (c *Context) newScopeCommon(id uid.UID, scopeType string, network object.NetworkReference, scopeData *ScopeData) (*Scope, error) {
	defer trace.End(trace.Begin(""))

	newScope := newScope(id, scopeType, network, scopeData)
	newScope.spaces = make([]*AddressSpace, len(scopeData.Pools))
	for i, p := range scopeData.Pools {
		r := ip.ParseRange(p)
		if r == nil {
			return nil, fmt.Errorf("invalid pool %s specified for scope %s", p, scopeData.Name)
		}

		newScope.spaces[i] = NewAddressSpaceFromRange(r.FirstIP, r.LastIP)
	}

	for k, v := range scopeData.Annotations {
		newScope.annotations[k] = v
	}

	if err := c.addScope(newScope); err != nil {
		return nil, err
	}

	return newScope, nil
}

func (c *Context) newBridgeScope(id uid.UID, scopeData *ScopeData) (newScope *Scope, err error) {
	defer trace.End(trace.Begin(""))
	bnPG, ok := c.config.PortGroups[c.config.BridgeNetwork]
	if !ok || bnPG == nil {
		return nil, fmt.Errorf("bridge network not set")
	}

	if ip.IsUnspecifiedSubnet(scopeData.Subnet) {
		// get the next available subnet from the default bridge pool
		var err error
		scopeData.Subnet, err = c.defaultBridgePool.NextIP4Net(c.defaultBridgeMask)
		if err != nil {
			return nil, err
		}
	}

	s, err := c.newScopeCommon(id, constants.BridgeScopeType, bnPG, scopeData)
	if err != nil {
		return nil, err
	}

	// add the gateway address to the bridge interface
	if err = c.config.BridgeLink.AddrAdd(net.IPNet{IP: s.Gateway(), Mask: s.Subnet().Mask}); err != nil {
		if errno, ok := err.(syscall.Errno); !ok || errno != syscall.EEXIST {
			log.Warnf("failed to add gateway address %s to bridge interface: %s", s.Gateway(), err)
		}
	}

	return s, nil
}

func (c *Context) newExternalScope(id uid.UID, scopeData *ScopeData) (*Scope, error) {
	defer trace.End(trace.Begin(""))

	// ipam cannot be specified without gateway and subnet
	if len(scopeData.Pools) > 0 {
		if ip.IsUnspecifiedSubnet(scopeData.Subnet) || scopeData.Gateway.IsUnspecified() {
			return nil, fmt.Errorf("ipam cannot be specified without gateway and subnet for external network")
		}
	}

	if !ip.IsUnspecifiedSubnet(scopeData.Subnet) {
		// cannot overlap with the default bridge pool
		if c.defaultBridgePool.Network.Contains(scopeData.Subnet.IP) ||
			c.defaultBridgePool.Network.Contains(highestIP4(scopeData.Subnet)) {
			return nil, fmt.Errorf("external network cannot overlap with default bridge network")
		}
	}

	pg := c.config.PortGroups[scopeData.Name]
	if pg == nil {
		return nil, fmt.Errorf("no network info for external scope %s", scopeData.Name)
	}

	return c.newScopeCommon(id, constants.ExternalScopeType, pg, scopeData)
}

func (c *Context) reserveSubnet(subnet *net.IPNet) (*AddressSpace, bool, error) {
	defer trace.End(trace.Begin(""))
	err := c.checkNetOverlap(subnet)
	if err != nil {
		return nil, false, err
	}

	// reserve from the default pool first
	space, err := c.defaultBridgePool.ReserveIP4Net(subnet)
	if err == nil {
		return space, true, nil
	}

	space = NewAddressSpaceFromNetwork(subnet)
	return space, false, nil
}

func (c *Context) checkNetOverlap(subnet *net.IPNet) error {
	// check if the requested subnet is available
	highestIP := highestIP4(subnet)
	for _, scope := range c.scopes {
		if scope.subnet.Contains(subnet.IP) || scope.subnet.Contains(highestIP) {
			return fmt.Errorf("subnet %s overlaps with scope %s subnet %s", subnet, scope.Name(), scope.Subnet())
		}
	}

	return nil
}

func reservePools(space *AddressSpace, pools []*AddressSpace) ([]*AddressSpace, error) {
	defer trace.End(trace.Begin(""))
	if len(pools) == 0 {
		// pool not specified so use the entire space
		return []*AddressSpace{space}, nil
	}

	var err error
	subSpaces := make([]*AddressSpace, len(pools))
	defer func() {
		if err == nil {
			return
		}

		for _, s := range subSpaces {
			if s == nil {
				continue
			}
			space.ReleaseIP4Range(s)

		}
	}()

	for i, p := range pools {
		var ss *AddressSpace
		if p.Network != nil {
			ss, err = space.ReserveIP4Net(p.Network)
			if err != nil {
				return nil, err
			}

			subSpaces[i] = ss
			continue
		}

		ss, err = space.ReserveIP4Range(p.Pool.FirstIP, p.Pool.LastIP)
		if err != nil {
			return nil, err
		}

		subSpaces[i] = ss
	}

	if err != nil {
		return nil, err
	}

	return subSpaces, nil
}

func scopeKey(sn string) string {
	return fmt.Sprintf("context.scopes.%s", sn)
}

// ScopeData holds fields used to create a new scope
type ScopeData struct {
	ScopeType   string
	Name        string
	Subnet      *net.IPNet
	Gateway     net.IP
	DNS         []net.IP
	TrustLevel  executor.TrustLevel
	Pools       []string
	Annotations map[string]string
	Internal    bool
}

func (c *Context) NewScope(ctx context.Context, scopeData *ScopeData) (*Scope, error) {
	defer trace.End(trace.Begin(""))

	c.Lock()
	defer c.Unlock()

	s, err := c.newScope(scopeData)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err != nil {
			c.deleteScope(s)
		}
	}()

	// save the scope in the kv store
	if c.kv != nil {
		var d []byte
		d, err = s.MarshalJSON()
		if err != nil {
			return nil, err
		}

		if err = c.kv.Put(ctx, scopeKey(s.Name()), d); err != nil {
			return nil, err
		}
	}

	return s, nil
}

func (c *Context) newScope(scopeData *ScopeData) (*Scope, error) {
	// sanity checks
	if scopeData.Name == "" {
		return nil, fmt.Errorf("scope name must not be empty")
	}

	if scopeData.Gateway == nil {
		scopeData.Gateway = net.IPv4(0, 0, 0, 0)
	}

	if _, ok := c.scopes[scopeData.Name]; ok {
		return nil, DuplicateResourceError{resID: scopeData.Name}
	}

	var s *Scope
	var err error
	switch scopeData.ScopeType {
	case constants.BridgeScopeType:
		s, err = c.newBridgeScope(uid.New(), scopeData)

	case constants.ExternalScopeType:
		s, err = c.newExternalScope(uid.New(), scopeData)

	default:
		return nil, fmt.Errorf("scope type not supported")
	}

	if err != nil {
		return nil, err
	}

	return s, nil
}

func (c *Context) findScopes(idName *string) ([]*Scope, error) {
	defer trace.End(trace.Begin(""))

	if idName != nil && *idName != "" {
		if *idName == "default" {
			return []*Scope{c.DefaultScope()}, nil
		}

		// search by name
		scope, ok := c.scopes[*idName]
		if ok {
			return []*Scope{scope}, nil
		}

		// search by id or partial id
		var ss []*Scope
		for _, s := range c.scopes {
			if strings.HasPrefix(s.id.String(), *idName) {
				ss = append(ss, s)
			}
		}

		if len(ss) > 0 {
			return ss, nil
		}

		return nil, ResourceNotFoundError{error: fmt.Errorf("scope %s not found", *idName)}
	}

	_scopes := make([]*Scope, len(c.scopes))
	// list all scopes
	i := 0
	for _, scope := range c.scopes {
		_scopes[i] = scope
		i++
	}

	return _scopes, nil
}

func (c *Context) Scopes(ctx context.Context, idName *string) ([]*Scope, error) {
	defer trace.End(trace.Begin(""))

	c.Lock()
	defer c.Unlock()

	scopes, err := c.findScopes(idName)
	if err != nil {
		return nil, err
	}

	// collate the containers to update
	containers := make(map[uid.UID]*Container)
	for _, s := range scopes {
		if !s.isDynamic() {
			continue
		}

		for _, c := range s.Containers() {
			containers[c.ID()] = c
		}
	}

	for _, c := range containers {
		c.Refresh(ctx)
	}

	return scopes, nil
}

func (c *Context) DefaultScope() *Scope {
	return c.defaultScope
}

func (c *Context) BindContainer(h *exec.Handle) ([]*Endpoint, error) {
	defer trace.End(trace.Begin(""))
	c.Lock()
	defer c.Unlock()

	return c.bindContainer(h)
}

func (c *Context) bindContainer(h *exec.Handle) ([]*Endpoint, error) {
	con, err := c.container(h)
	if con != nil {
		return con.Endpoints(), nil // already bound
	}

	if _, ok := err.(ResourceNotFoundError); !ok {
		return nil, err
	}

	con = &Container{
		id:   uid.Parse(h.ExecConfig.ID),
		name: h.ExecConfig.Name,
	}

	defaultMarked := false
	aliases := make(map[string]*Container)
	var endpoints []*Endpoint
	for _, ne := range h.ExecConfig.Networks {
		var s *Scope
		s, ok := c.scopes[ne.Network.Name]
		if !ok {
			return nil, &ResourceNotFoundError{error: fmt.Errorf("network %s not found", ne.Network.Name)}
		}

		defer func() {
			if err == nil {
				return
			}

			s.RemoveContainer(con)
		}()

		var eip *net.IP
		if ne.Static {
			eip = &ne.IP.IP
		} else if !ip.IsUnspecifiedIP(ne.Assigned.IP) {
			// for VCH restart, we need to reserve
			// the IP of the running container
			//
			// this may be a DHCP assigned IP, however, the
			// addContainer call below will ignore reserving
			// an IP if the scope is "dynamic"
			eip = &ne.Assigned.IP
		}

		e := newEndpoint(con, s, eip, nil)
		e.static = ne.Static
		if err = s.AddContainer(con, e); err != nil {
			return nil, err
		}

		ports, _, err := nat.ParsePortSpecs(ne.Ports)
		if err != nil {
			return nil, err
		}
		for p := range ports {
			var port Port
			if port, err = ParsePort(string(p)); err != nil {
				return nil, err
			}

			if err = e.addPort(port); err != nil {
				return nil, err
			}
		}

		if !ip.IsUnspecifiedIP(e.IP()) {
			ne.IP = &net.IPNet{
				IP:   e.IP(),
				Mask: e.Scope().Subnet().Mask,
			}
		}
		ne.Network.Gateway = net.IPNet{IP: e.Gateway(), Mask: e.Subnet().Mask}
		ne.Network.Nameservers = make([]net.IP, len(s.dns))
		copy(ne.Network.Nameservers, s.dns)

		// mark the external network as default
		scope := e.Scope()
		if !defaultMarked && scope.Type() == constants.ExternalScopeType {
			defaultMarked = true
			ne.Network.Default = true
		}

		if scope.Internal() {
			ne.Network.Default = false
		}

		// dns lookup aliases
		aliases[fmt.Sprintf("%s:%s", s.Name(), con.name)] = con
		aliases[fmt.Sprintf("%s:%s", s.Name(), con.id.Truncate())] = con

		// container specific aliases
		for _, a := range ne.Network.Aliases {
			log.Debugf("parsing alias %s", a)
			l := strings.Split(a, ":")
			if len(l) != 2 {
				err = fmt.Errorf("Parsing network alias %s failed", a)
				return nil, err
			}

			who, what := l[0], l[1]
			if who == "" {
				who = con.name
			}
			if a, exists := e.addAlias(who, what); a != badAlias && !exists {
				whoc := con
				// if the alias is not for this container, then
				// find it in the container collection
				if who != con.name {
					whoc = c.containers[who]
				}

				// whoc may be nil here, which means that the aliased
				// container is not bound yet; this is OK, and will be
				// fixed up when "who" is bound
				if whoc != nil {
					aliases[a.scopedName()] = whoc
				} else {
					log.Debugf("skipping alias %s since %s is not bound yet", a, who)
				}
			}
		}

		// fix up the aliases to this container
		// from other containers
		for _, e := range s.Endpoints() {
			if e.Container() == con {
				continue
			}

			log.Debugf("getting aliases for %s from %s", con.name, e.Container().Name())
			for _, a := range e.getAliases(con.name) {
				aliases[a.scopedName()] = con
			}
		}

		endpoints = append(endpoints, e)
	}

	// FIXME: if there was no external network to mark as default,
	// then just pick the first network to mark as default
	if !defaultMarked {
		defaultMarked = true
		for _, ne := range h.ExecConfig.Networks {

			if s, ok := c.scopes[ne.Network.Name]; ok && s.Internal() {
				log.Debugf("not setting internal network %s as default", ne.Network.Name)
				continue
			}

			ne.Network.Default = true
			break
		}
	}

	// long id
	c.containers[con.id.String()] = con
	// short id
	c.containers[con.id.Truncate().String()] = con
	// name
	c.containers[con.name] = con
	// aliases
	for k, v := range aliases {
		log.Debugf("adding alias %s -> %s", k, v.Name())
		cons := c.aliases[k]
		found := false
		for _, c := range cons {
			if v == c {
				found = true
				break
			}
		}
		if !found {
			c.aliases[k] = append(cons, v)
		}
	}

	return endpoints, nil
}

func (c *Context) container(h *exec.Handle) (*Container, error) {
	defer trace.End(trace.Begin(""))
	id := uid.Parse(h.ExecConfig.ID)
	if id == uid.NilUID {
		return nil, fmt.Errorf("invalid container id %s", h.ExecConfig.ID)
	}

	if con, ok := c.containers[id.String()]; ok {
		return con, nil
	}

	return nil, ResourceNotFoundError{error: fmt.Errorf("container %s not found", id.String())}
}

func (c *Context) UnbindContainer(h *exec.Handle) ([]*Endpoint, error) {
	defer trace.End(trace.Begin(""))
	c.Lock()
	defer c.Unlock()

	con, err := c.container(h)
	if err != nil {
		if _, ok := err.(ResourceNotFoundError); ok {
			return nil, nil // not bound
		}

		return nil, err
	}

	// aliases to remove
	var aliases []string
	var endpoints []*Endpoint
	for _, ne := range h.ExecConfig.Networks {
		var s *Scope
		s, ok := c.scopes[ne.Network.Name]
		if !ok {
			return nil, &ResourceNotFoundError{}
		}

		// save the endpoint info
		e := con.Endpoint(s).copy()

		if err = s.RemoveContainer(con); err != nil {
			return nil, err
		}

		// clear out assigned ip
		ne.Assigned.IP = net.IPv4zero

		// aliases to remove
		// name for dns lookup
		aliases = append(aliases, fmt.Sprintf("%s:%s", s.Name(), con.name))
		aliases = append(aliases, fmt.Sprintf("%s:%s", s.Name(), con.id.Truncate()))
		for _, as := range e.aliases {
			for _, a := range as {
				aliases = append(aliases, a.scopedName())
			}
		}

		// aliases from other containers
		for _, e := range s.Endpoints() {
			if e.Container() == con {
				continue
			}

			for _, a := range e.getAliases(con.name) {
				aliases = append(aliases, a.scopedName())
			}
		}

		endpoints = append(endpoints, e)
	}

	// remove aliases
	for _, a := range aliases {
		as := c.aliases[a]
		for i := range as {
			if as[i] == con {
				as = append(as[:i], as[i+1:]...)
				if len(as) == 0 {
					delete(c.aliases, a)
				} else {
					c.aliases[a] = as
				}

				break
			}
		}
	}

	// long id
	delete(c.containers, con.ID().String())
	// short id
	delete(c.containers, con.ID().Truncate().String())
	// name
	delete(c.containers, con.Name())

	return endpoints, nil
}

var addEthernetCard = func(h *exec.Handle, s *Scope) (types.BaseVirtualDevice, error) {
	var devices object.VirtualDeviceList
	var d types.BaseVirtualDevice
	var dc types.BaseVirtualDeviceConfigSpec

	ctx := context.Background()
	dcs, err := h.Spec.FindNICs(ctx, s.Network())
	if err != nil {
		return nil, err
	}

	for _, ds := range dcs {
		if ds.GetVirtualDeviceConfigSpec().Operation == types.VirtualDeviceConfigSpecOperationAdd {
			d = ds.GetVirtualDeviceConfigSpec().Device
			dc = ds
			break
		}
	}

	if d == nil {
		backing, err := s.Network().EthernetCardBackingInfo(ctx)
		if err != nil {
			return nil, err
		}

		if d, err = devices.CreateEthernetCard("vmxnet3", backing); err != nil {
			return nil, err
		}

		d.GetVirtualDevice().DeviceInfo = &types.Description{
			Label: s.name,
		}
	}

	if spec.VirtualDeviceSlotNumber(d) == spec.NilSlot {
		slots := make(map[int32]bool)
		for _, e := range h.ExecConfig.Networks {
			if e.Common.ID != "" {
				slot, err := strconv.Atoi(e.Common.ID)
				if err == nil {
					slots[int32(slot)] = true
				}
			}
		}

		h.Spec.AssignSlotNumber(d, slots)
	}

	if dc == nil {
		devices = append(devices, d)
		deviceSpecs, err := devices.ConfigSpec(types.VirtualDeviceConfigSpecOperationAdd)
		if err != nil {
			return nil, err
		}

		h.Spec.DeviceChange = append(h.Spec.DeviceChange, deviceSpecs...)
	}

	return d, nil
}

func (c *Context) resolveScope(scope string) (*Scope, error) {
	scopes, err := c.findScopes(&scope)
	if err != nil || len(scopes) != 1 {
		return nil, err
	}

	return scopes[0], nil
}

// AddContainer add a container to the specified scope, optionally specifying an ip address
// for the container in the scope
func (c *Context) AddContainer(h *exec.Handle, options *AddContainerOptions) error {
	defer trace.End(trace.Begin(""))
	c.Lock()
	defer c.Unlock()

	if h == nil {
		return fmt.Errorf("handle is required")
	}

	var err error
	s, err := c.resolveScope(options.Scope)
	if err != nil {
		return err
	}

	if h.ExecConfig.Networks != nil {
		if _, ok := h.ExecConfig.Networks[s.Name()]; ok {
			// already part of this scope
			return nil
		}

		// check if container is already part of an "external" scope;
		// only one "external" scope per container is allowed
		if s.Type() == constants.ExternalScopeType {
			for name := range h.ExecConfig.Networks {
				sc, _ := c.resolveScope(name)
				if sc.Type() == constants.ExternalScopeType {
					return fmt.Errorf("container can only be added to at most one mapped network")
				}
			}
		}
	}

	if s.Type() == constants.ExternalScopeType {
		// Check this isn't a port mapping.  On an external network, we
		// aren't doing any PAT'ing.  We're simply unblocking that port on
		// the cVM.
		for _, p := range options.Ports {
			if strings.Contains(p, ":") {
				err = fmt.Errorf("external scope includes a port mapping (%s)", p)
				log.Errorln(err)
				return err
			}
		}
		// Check that ports are only opened on published network firewall configuration.
		if len(options.Ports) > 0 && s.TrustLevel() == executor.Closed {
			err = fmt.Errorf("Ports cannot be published via the \"closed\" container network firewall.")
			log.Errorln(err)
			return err
		}
	}

	// figure out if we need to add a new NIC
	// if there is already a NIC connected to a
	// bridge network and we are adding the container
	// to a bridge network, we just reuse that
	// NIC
	var pciSlot int32
	if s.Type() == constants.BridgeScopeType {
		for _, ne := range h.ExecConfig.Networks {
			sc, err := c.resolveScope(ne.Network.Name)
			if err != nil {
				return err
			}

			if sc.Type() != constants.BridgeScopeType {
				continue
			}

			if ne.ID != "" {
				pciSlot = atoiOrZero(ne.ID)
				if pciSlot != 0 {
					break
				}
			}
		}
	}

	if pciSlot == 0 {
		d, err := addEthernetCard(h, s)
		if err != nil {
			return err
		}

		pciSlot = spec.VirtualDeviceSlotNumber(d)
	}

	if h.ExecConfig.Networks == nil {
		h.ExecConfig.Networks = make(map[string]*executor.NetworkEndpoint)
	}

	ne := &executor.NetworkEndpoint{
		Common: executor.Common{
			ID: strconv.Itoa(int(pciSlot)),
		},
		Network: executor.ContainerNetwork{
			Common: executor.Common{
				Name: s.Name(),
			},
			Aliases:    options.Aliases,
			Type:       s.Type(),
			TrustLevel: s.TrustLevel(),
		},
		Ports: options.Ports,
	}
	pools := s.Pools()
	ne.Network.Pools = make([]ip.Range, len(pools))
	for i, p := range pools {
		ne.Network.Pools[i] = *p
	}

	ne.Static = false
	if len(options.IP) > 0 && !ip.IsUnspecifiedIP(options.IP) {
		ne.Static = true
		ne.IP = &net.IPNet{
			IP:   options.IP,
			Mask: s.Subnet().Mask,
		}
	}

	h.ExecConfig.Networks[s.Name()] = ne
	return nil
}

func (c *Context) RemoveContainer(h *exec.Handle, scope string) error {
	defer trace.End(trace.Begin(""))
	c.Lock()
	defer c.Unlock()

	if h == nil {
		return fmt.Errorf("handle is required")
	}

	if con, _ := c.container(h); con != nil {
		return fmt.Errorf("container is bound")
	}

	var err error
	s, err := c.resolveScope(scope)
	if err != nil {
		return err
	}

	var ne *executor.NetworkEndpoint
	ne, ok := h.ExecConfig.Networks[s.Name()]
	if !ok {
		return fmt.Errorf("container %s not part of network %s", h.ExecConfig.ID, s.Name())
	}

	// figure out if any other networks are using the NIC
	removeNIC := true
	for _, ne2 := range h.ExecConfig.Networks {
		if ne2 == ne {
			continue
		}
		if ne2.ID == ne.ID {
			removeNIC = false
			break
		}
	}

	if removeNIC {
		var devices object.VirtualDeviceList
		backing, err := s.network.EthernetCardBackingInfo(context.Background())
		if err != nil {
			return err
		}

		d, err := devices.CreateEthernetCard("vmxnet3", backing)
		if err != nil {
			return err
		}

		devices = append(devices, d)
		spec, err := devices.ConfigSpec(types.VirtualDeviceConfigSpecOperationRemove)
		if err != nil {
			return err
		}
		h.Spec.DeviceChange = append(h.Spec.DeviceChange, spec...)
	}

	delete(h.ExecConfig.Networks, s.Name())

	return nil
}

func (c *Context) Container(key string) *Container {
	defer trace.End(trace.Begin(""))

	c.Lock()
	defer c.Unlock()

	log.Debugf("container lookup for %s", key)
	if con, ok := c.containers[key]; ok {
		return con
	}

	return nil
}

func (c *Context) ContainersByAlias(alias string) []*Container {
	defer trace.End(trace.Begin(""))

	c.Lock()
	defer c.Unlock()

	if cons, ok := c.aliases[alias]; ok {
		log.Debugf("cons=%#v", cons)
		return cons
	}

	return nil
}

func (c *Context) ContainerByAddr(addr net.IP) *Endpoint {
	defer trace.End(trace.Begin(""))

	c.Lock()
	defer c.Unlock()

	for _, s := range c.scopes {
		if e := s.ContainerByAddr(addr); e != nil {
			return e
		}
	}

	return nil
}

func (c *Context) DeleteScope(ctx context.Context, name string) error {
	defer trace.End(trace.Begin(""))

	c.Lock()
	defer c.Unlock()

	s, err := c.resolveScope(name)
	if err != nil {
		return err
	}

	if s == nil {
		return ResourceNotFoundError{}
	}

	if s.builtin {
		return fmt.Errorf("cannot remove builtin scope")
	}

	if len(s.Endpoints()) != 0 {
		return fmt.Errorf("%s has active endpoints", s.Name())
	}

	if c.kv != nil {
		if err = c.kv.Delete(ctx, scopeKey(s.Name())); err != nil && err != kvstore.ErrKeyNotFound {
			return err
		}
	}

	c.deleteScope(s)
	return nil
}

func (c *Context) deleteScope(s *Scope) {
	if s.Type() == constants.BridgeScopeType {
		// remove gateway ip from bridge interface
		addr := net.IPNet{IP: s.Gateway(), Mask: s.Subnet().Mask}
		if err := c.config.BridgeLink.AddrDel(addr); err != nil {
			if errno, ok := err.(syscall.Errno); !ok || errno != syscall.EADDRNOTAVAIL {
				log.Warnf("could not remove gateway address %s for scope %s on link %s: %s", addr, s.Name(), c.config.BridgeLink.Attrs().Name, err)
			}
		}
	}

	delete(c.scopes, s.Name())
}

func atoiOrZero(a string) int32 {
	i, _ := strconv.Atoi(a)
	return int32(i)
}
