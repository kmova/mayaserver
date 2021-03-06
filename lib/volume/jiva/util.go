// This file handles jiva storage logic related to mayaserver's orchestration
// provider.
//
// NOTE:
//    jiva storage delegates the provisioning, placement & other operational
// aspects to an orchestration provider. Some of the orchestration providers
// can be Kubernetes, Nomad, etc.
package jiva

import (
	"fmt"

	"github.com/openebs/mayaserver/lib/api/v1"
	v1jiva "github.com/openebs/mayaserver/lib/api/v1/jiva"
	"github.com/openebs/mayaserver/lib/nethelper"
	"github.com/openebs/mayaserver/lib/volume"
)

type JivaInterface interface {

	// Name provides the name of the JivaInterface implementor
	Name() string

	// This is a builder method for NetworkOps. It will return false
	// if Network operations is not supported.
	NetworkOps() (NetworkOps, bool)

	// This is a builder method for StorageOps. It will return false
	// if Storage operations is not supported.
	StorageOps() (StorageOps, bool)
}

type NetworkOps interface {
	NetworkInfo(dc string) (map[v1.ContainerNetworkingLbl]string, error)
}

type StorageOps interface {
	StorageInfo(*v1.PersistentVolumeClaim) (*v1.PersistentVolume, error)

	ProvisionStorage(*v1.PersistentVolumeClaim) (*v1.PersistentVolume, error)

	DeleteStorage(*v1.PersistentVolume) (*v1.PersistentVolume, error)
}

// jivaUtil is the concrete implementation for
//
//  1. JivaInterface interface
//  2. NetworkOps interface
//  3. StorageOps interface
type jivaUtil struct {
	// Orthogonal concerns and their management w.r.t jiva storage
	// is done via aspect
	aspect volume.VolumePluginAspect
}

// newJivaUtil provides a orchestrator based infrastructure that
// supports jiva operations
func newJivaUtil(aspect volume.VolumePluginAspect) (JivaInterface, error) {
	if aspect == nil {
		return nil, fmt.Errorf("Nil volume plugin aspect was provided")
	}

	return &jivaUtil{
		aspect: aspect,
	}, nil
}

// This is a plain jiva utility implementation. Hence the name.
func (j *jivaUtil) Name() string {
	return "jivautil"
}

// StorageOps method provides an instance of StorageOps interface
//
// NOTE:
//  jivaUtil implements StorageOps interface. Hence it returns self.
func (j *jivaUtil) StorageOps() (StorageOps, bool) {
	return j, true
}

// NetworkOps method provides an instance of NetworkOps interface
//
// NOTE:
//  jivaUtil implements NetworkOps interface. Hence it returns self.
func (j *jivaUtil) NetworkOps() (NetworkOps, bool) {
	return j, true
}

// NetworkInfo tries to fetch networking details from its orchestrator
//
// NOTE:
//  This is a concrete implementation of NetworkOps interface
func (j *jivaUtil) NetworkInfo(dc string) (map[v1.ContainerNetworkingLbl]string, error) {

	orchestrator, err := j.aspect.GetOrchProvider()
	if err != nil {
		return nil, err
	}

	networkOrchestrator, ok := orchestrator.NetworkPlacements()

	if !ok {
		return nil, fmt.Errorf("Network operations not supported by orchestrator '%s'", orchestrator.Name())
	}

	return networkOrchestrator.NetworkInfoReq(dc)
}

// Info tries to fetch details of a jiva volume placed in an orchestrator
func (j *jivaUtil) StorageInfo(pvc *v1.PersistentVolumeClaim) (*v1.PersistentVolume, error) {

	orchestrator, err := j.aspect.GetOrchProvider()
	if err != nil {
		return nil, err
	}

	storageOrchestrator, ok := orchestrator.StoragePlacements()

	if !ok {
		return nil, fmt.Errorf("Storage operations not supported by orchestrator '%s'", orchestrator.Name())
	}

	return storageOrchestrator.StorageInfoReq(pvc)
}

// Provision tries to creates a jiva volume via an orchestrator
func (j *jivaUtil) ProvisionStorage(pvc *v1.PersistentVolumeClaim) (*v1.PersistentVolume, error) {

	orchestrator, err := j.aspect.GetOrchProvider()
	if err != nil {
		return nil, err
	}

	storageOrchestrator, ok := orchestrator.StoragePlacements()

	if !ok {
		return nil, fmt.Errorf("Storage operations not supported by orchestrator '%s'", orchestrator.Name())
	}

	err = initLabels(pvc)
	if err != nil {
		return nil, err
	}

	err = setJivaProps(pvc)
	if err != nil {
		return nil, err
	}

	err = j.setRegion(pvc)
	if err != nil {
		return nil, err
	}

	dc, err := j.setDC(pvc)
	if err != nil {
		return nil, err
	}

	err = j.setCN(dc, pvc)
	if err != nil {
		return nil, err
	}

	return storageOrchestrator.StoragePlacementReq(pvc)
}

// initLabels is a utility function that will initialize the Labels
// of a PersistentVolumeClaim if not done so already.
func initLabels(pvc *v1.PersistentVolumeClaim) error {

	// return if already initialized
	if pvc.Labels != nil {
		return nil
	}

	// initialize with an empty list
	pvc.Labels = map[string]string{}

	return nil
}

// setJivaProps function sets jiva specific properties with defaults
// if not done so already.
func setJivaProps(pvc *v1.PersistentVolumeClaim) error {

	if pvc.Labels == nil {
		return fmt.Errorf("Persistent volume claim's labels not initialized")
	}

	if pvc.Labels[string(v1jiva.JivaFrontEndImageLbl)] == "" {
		pvc.Labels[string(v1jiva.JivaFrontEndImageLbl)] = "openebs/jiva:latest"
	}

	return nil
}

// setRegion sets the region property of a PersistentVolumeClaim
// if not done so already.
func (j *jivaUtil) setRegion(pvc *v1.PersistentVolumeClaim) error {

	if pvc.Labels == nil {
		return fmt.Errorf("Persistent volume claim's labels not initialized")
	}

	// return if region is already set
	if pvc.Labels[string(v1.RegionLbl)] != "" {
		return nil
	}

	if j.aspect == nil {
		return fmt.Errorf("Aspect missing while setting pvc region")
	}

	o, err := j.aspect.GetOrchProvider()
	if err != nil {
		return err
	}

	if o == nil {
		return fmt.Errorf("Orchestrator missing while setting pvc region")
	}

	// Set the pvc's region from jiva's orchestrator
	region := o.Region()
	if region == "" {
		return fmt.Errorf("Region could not be determined")
	}

	// set dc in pvc
	pvc.Labels[string(v1.RegionLbl)] = region

	return nil
}

// setDC sets the datacenter property of a PersistentVolumeClaim
// if not done so already.
func (j *jivaUtil) setDC(pvc *v1.PersistentVolumeClaim) (string, error) {

	if pvc.Labels == nil {
		return "", fmt.Errorf("Persistent volume claim's labels not initialized")
	}

	// return if dc is already set
	if pvc.Labels[string(v1.DatacenterLbl)] != "" {
		return pvc.Labels[string(v1.DatacenterLbl)], nil
	}

	// Set the pvc with dc from jiva's aspect
	dc, err := j.aspect.DefaultDatacenter()
	if err != nil {
		return "", err
	}

	if dc == "" {
		return "", fmt.Errorf("Datacenter could not be determined")
	}

	// set dc in pvc
	pvc.Labels[string(v1.DatacenterLbl)] = dc

	return dc, nil
}

// setCN sets the container networking properties in a PersistentVolumeClaim
// if not done so already.
func (j *jivaUtil) setCN(dc string, pvc *v1.PersistentVolumeClaim) error {

	if pvc.Labels == nil {
		return fmt.Errorf("Persistent volume claim's labels not initialized")
	}

	if dc == "" {
		return fmt.Errorf("Datacenter not provided")
	}

	// Fetch the networking options that are orchestrator & datacenter specific
	cn, err := j.NetworkInfo(dc)
	if err != nil {
		return err
	}

	// Set the networking options if not already set
	//
	// NOTE:
	//    User provided networking options score over
	// orchestrator & dc specific configurations
	for k, _ := range cn {
		if pvc.Labels[string(k)] == "" {
			pvc.Labels[string(k)] = cn[k]
		}
	}

	networkCIDR := pvc.Labels[string(v1.CNNetworkCIDRAddrLbl)]
	if networkCIDR == "" {
		return fmt.Errorf("Network CIDR could not be determined")
	}

	subnet, err := nethelper.CIDRSubnet(networkCIDR)
	if err != nil {
		return err
	}

	pvc.Labels[string(v1.CNSubnetLbl)] = subnet

	// TODO
	// Below is a tightly coupled design which makes sense of only
	// one jiva controller & one jiva replica.
	if pvc.Labels[string(v1jiva.JivaFrontEndIPLbl)] == "" && pvc.Labels[string(v1jiva.JivaBackEndIPLbl)] == "" {
		// Get two available IPs
		ips, err := nethelper.GetAvailableIPs(networkCIDR, 2)
		if err != nil {
			return err
		}

		pvc.Labels[string(v1jiva.JivaFrontEndIPLbl)] = ips[0]
		pvc.Labels[string(v1jiva.JivaBackEndIPLbl)] = ips[1]

		return nil
	}

	if pvc.Labels[string(v1jiva.JivaFrontEndIPLbl)] == "" {
		// Get one available IP
		ips, err := nethelper.GetAvailableIPs(networkCIDR, 1)
		if err != nil {
			return err
		}

		pvc.Labels[string(v1jiva.JivaFrontEndIPLbl)] = ips[0]

		return nil
	}

	if pvc.Labels[string(v1jiva.JivaBackEndIPLbl)] == "" {
		// Get one available IP
		ips, err := nethelper.GetAvailableIPs(networkCIDR, 1)
		if err != nil {
			return err
		}

		pvc.Labels[string(v1jiva.JivaBackEndIPLbl)] = ips[0]

		return nil
	}

	return nil
}

// Delete tries to delete the jiva volume via an orchestrator
func (j *jivaUtil) DeleteStorage(pv *v1.PersistentVolume) (*v1.PersistentVolume, error) {
	orchestrator, err := j.aspect.GetOrchProvider()
	if err != nil {
		return nil, err
	}

	storageOrchestrator, ok := orchestrator.StoragePlacements()

	if !ok {
		return nil, fmt.Errorf("Storage operations not supported by orchestrator '%s'", orchestrator.Name())
	}

	return storageOrchestrator.StorageRemovalReq(pv)
}
