package nomad

import (
	"fmt"
	"strconv"
	"time"

	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/helper"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/openebs/mayaserver/lib/api/v1"
	v1jiva "github.com/openebs/mayaserver/lib/api/v1/jiva"
)

// Get the job name from a persistent volume claim
func PvcToJobName(pvc *v1.PersistentVolumeClaim) (string, error) {

	if pvc == nil {
		return "", fmt.Errorf("Nil persistent volume claim provided")
	}

	if pvc.Name == "" {
		return "", fmt.Errorf("Missing name in persistent volume claim")
	}

	return pvc.Name, nil
}

// Transform a PersistentVolumeClaim type to Nomad job type
func PvcToJob(pvc *v1.PersistentVolumeClaim) (*api.Job, error) {

	if pvc == nil {
		return nil, fmt.Errorf("Nil persistent volume claim provided")
	}

	if pvc.Name == "" {
		return nil, fmt.Errorf("Missing name in persistent volume claim")
	}

	if pvc.Labels == nil {
		return nil, fmt.Errorf("Missing labels in persistent volume claim")
	}

	if pvc.Labels[string(v1.RegionLbl)] == "" {
		return nil, fmt.Errorf("Missing region in persistent volume claim")
	}

	if pvc.Labels[string(v1.DatacenterLbl)] == "" {
		return nil, fmt.Errorf("Missing datacenter in persistent volume claim")
	}

	if pvc.Labels[string(v1jiva.JivaFrontEndImageLbl)] == "" {
		return nil, fmt.Errorf("Missing jiva fe image version in persistent volume claim")
	}

	if pvc.Labels[string(v1.CNTypeLbl)] == "" {
		return nil, fmt.Errorf("Missing cn type in persistent volume claim")
	}

	if pvc.Labels[string(v1jiva.JivaFrontEndIPLbl)] == "" {
		return nil, fmt.Errorf("Missing jiva fe ip in persistent volume claim")
	}

	if pvc.Labels[string(v1jiva.JivaBackEndIPLbl)] == "" {
		return nil, fmt.Errorf("Missing jiva be ip in persistent volume claim")
	}

	if pvc.Labels[string(v1.CNSubnetLbl)] == "" {
		return nil, fmt.Errorf("Missing cn subnet in persistent volume claim")
	}

	if pvc.Labels[string(v1.CNInterfaceLbl)] == "" {
		return nil, fmt.Errorf("Missing cn interface in persistent volume claim")
	}

	// TODO
	// ID is same as Name currently
	// Do we need to think on it ?
	jobName := helper.StringToPtr(pvc.Name)
	region := helper.StringToPtr(pvc.Labels[string(v1.RegionLbl)])
	dc := pvc.Labels[string(v1.DatacenterLbl)]

	jivaGroupName := "pod"
	jivaVolName := pvc.Name

	// TODO
	// Get from the PVC
	jivaVolSize := "5g"

	feTaskGroup := "fe" + jivaGroupName
	feTaskName := "fe1"
	beTaskGroup := "be" + jivaGroupName
	beTaskName := "be1"

	jivaFeVersion := pvc.Labels[string(v1jiva.JivaFrontEndImageLbl)]
	jivaNetworkType := pvc.Labels[string(v1.CNTypeLbl)]
	jivaFeIP := pvc.Labels[string(v1jiva.JivaFrontEndIPLbl)]
	jivaBeIP := pvc.Labels[string(v1jiva.JivaBackEndIPLbl)]
	jivaFeSubnet := pvc.Labels[string(v1.CNSubnetLbl)]
	jivaFeInterface := pvc.Labels[string(v1.CNInterfaceLbl)]

	// TODO
	// Transformation from pvc or pv to nomad types & vice-versa:
	//
	//  1. Need an Interface or functional callback defined at
	// lib/api/v1/nomad/ &
	//  2. implemented by the volume plugins that want
	// to be orchestrated by Nomad
	//  3. This transformer instance needs to be injected from
	// volume plugin to orchestrator, in a generic way.

	// Hardcoded logic all the way
	// Nomad specific defaults, hardcoding is OK.
	// However, volume plugin specific stuff is BAD
	return &api.Job{
		Region:      region,
		Name:        jobName,
		ID:          jobName,
		Datacenters: []string{dc},
		Type:        helper.StringToPtr(api.JobTypeService),
		Priority:    helper.IntToPtr(50),
		Constraints: []*api.Constraint{
			api.NewConstraint("${attr.kernel.name}", "=", "linux"),
		},
		// Meta information will be used to pass on the metadata from
		// nomad to clients of mayaserver.
		Meta: map[string]string{
			"targetportal": jivaFeIP + ":" + v1jiva.JivaIscsiTargetPortalPort,
			"iqn":          v1jiva.JivaIqnFormatPrefix + ":" + jivaVolName,
		},
		TaskGroups: []*api.TaskGroup{
			// jiva frontend
			&api.TaskGroup{
				Name:  helper.StringToPtr(feTaskGroup),
				Count: helper.IntToPtr(1),
				RestartPolicy: &api.RestartPolicy{
					Attempts: helper.IntToPtr(3),
					Interval: helper.TimeToPtr(5 * time.Minute),
					Delay:    helper.TimeToPtr(25 * time.Second),
					Mode:     helper.StringToPtr("delay"),
				},
				Tasks: []*api.Task{
					&api.Task{
						Name:   feTaskName,
						Driver: "raw_exec",
						Resources: &api.Resources{
							CPU:      helper.IntToPtr(500),
							MemoryMB: helper.IntToPtr(256),
							Networks: []*api.NetworkResource{
								&api.NetworkResource{
									MBits: helper.IntToPtr(400),
								},
							},
						},
						Env: map[string]string{
							"JIVA_CTL_NAME":    pvc.Name + "-" + feTaskGroup + "-" + feTaskName,
							"JIVA_CTL_VERSION": jivaFeVersion,
							"JIVA_CTL_VOLNAME": jivaVolName,
							"JIVA_CTL_VOLSIZE": jivaVolSize,
							"JIVA_CTL_IP":      jivaFeIP,
							"JIVA_CTL_SUBNET":  jivaFeSubnet,
							"JIVA_CTL_IFACE":   jivaFeInterface,
						},
						Artifacts: []*api.TaskArtifact{
							&api.TaskArtifact{
								GetterSource: helper.StringToPtr("https://raw.githubusercontent.com/openebs/jiva/master/scripts/launch-jiva-ctl-with-ip"),
								RelativeDest: helper.StringToPtr("local/"),
							},
						},
						Config: map[string]interface{}{
							"command": "launch-jiva-ctl-with-ip",
						},
						LogConfig: &api.LogConfig{
							MaxFiles:      helper.IntToPtr(3),
							MaxFileSizeMB: helper.IntToPtr(1),
						},
					},
				},
			},
			// jiva replica
			&api.TaskGroup{
				Name:  helper.StringToPtr(beTaskGroup),
				Count: helper.IntToPtr(1),
				RestartPolicy: &api.RestartPolicy{
					Attempts: helper.IntToPtr(3),
					Interval: helper.TimeToPtr(5 * time.Minute),
					Delay:    helper.TimeToPtr(25 * time.Second),
					Mode:     helper.StringToPtr("delay"),
				},
				Tasks: []*api.Task{
					&api.Task{
						Name:   beTaskName,
						Driver: "raw_exec",
						Resources: &api.Resources{
							CPU:      helper.IntToPtr(500),
							MemoryMB: helper.IntToPtr(256),
							Networks: []*api.NetworkResource{
								&api.NetworkResource{
									MBits: helper.IntToPtr(400),
								},
							},
						},
						Env: map[string]string{
							"JIVA_REP_NAME":     pvc.Name + "-" + beTaskGroup + "-" + beTaskName,
							"JIVA_CTL_IP":       jivaFeIP,
							"JIVA_REP_VOLNAME":  jivaVolName,
							"JIVA_REP_VOLSIZE":  jivaVolSize,
							"JIVA_REP_VOLSTORE": "/tmp/jiva/" + pvc.Name + beTaskGroup + "/" + beTaskName,
							"JIVA_REP_VERSION":  jivaFeVersion,
							"JIVA_REP_NETWORK":  jivaNetworkType,
							"JIVA_REP_IFACE":    jivaFeInterface,
							"JIVA_REP_IP":       jivaBeIP,
							"JIVA_REP_SUBNET":   jivaFeSubnet,
						},
						Artifacts: []*api.TaskArtifact{
							&api.TaskArtifact{
								GetterSource: helper.StringToPtr("https://raw.githubusercontent.com/openebs/jiva/master/scripts/launch-jiva-rep-with-ip"),
								RelativeDest: helper.StringToPtr("local/"),
							},
						},
						Config: map[string]interface{}{
							"command": "launch-jiva-rep-with-ip",
						},
						LogConfig: &api.LogConfig{
							MaxFiles:      helper.IntToPtr(3),
							MaxFileSizeMB: helper.IntToPtr(1),
						},
					},
				},
			},
		},
	}, nil
}

// TODO
// Transformation from JobSummary to pv
//
//  1. Need an Interface or functional callback defined at
// lib/api/v1/nomad.go &
//  2. implemented by the volume plugins that want
// to be orchestrated by Nomad
//  3. This transformer instance needs to be injected from
// volume plugin to orchestrator, in a generic way.
//func JobSummaryToPv(jobSummary *api.JobSummary) (*v1.PersistentVolume, error) {
//
//	if jobSummary == nil {
//		return nil, fmt.Errorf("Nil nomad job summary provided")
//	}
//
// TODO
// Needs to be filled up
//	return &v1.PersistentVolume{}, nil
//}

// TODO
// Transform the evaluation of a job to a PersistentVolume
func JobEvalToPv(jobName string, eval *api.Evaluation) (*v1.PersistentVolume, error) {

	if eval == nil {
		return nil, fmt.Errorf("Nil job evaluation provided")
	}

	pv := &v1.PersistentVolume{}
	pv.Name = jobName

	evalProps := map[string]string{
		"evalpriority":    strconv.Itoa(eval.Priority),
		"evaltype":        eval.Type,
		"evaltrigger":     eval.TriggeredBy,
		"evaljob":         eval.JobID,
		"evalstatus":      eval.Status,
		"evalstatusdesc":  eval.StatusDescription,
		"evalblockedeval": eval.BlockedEval,
	}
	pv.Annotations = evalProps

	pvs := v1.PersistentVolumeStatus{
		Message: eval.StatusDescription,
		Reason:  eval.Status,
	}
	pv.Status = pvs

	return pv, nil
}

// Transform a PersistentVolume type to Nomad Job type
func PvToJob(pv *v1.PersistentVolume) (*api.Job, error) {

	if pv == nil {
		return nil, fmt.Errorf("Nil persistent volume provided")
	}

	return &api.Job{
		Name: helper.StringToPtr(pv.Name),
		// TODO
		// ID is same as Name currently
		ID: helper.StringToPtr(pv.Name),
	}, nil
}

// Transform a Nomad Job to a PersistentVolume
func JobToPv(job *api.Job) (*v1.PersistentVolume, error) {
	if job == nil {
		return nil, fmt.Errorf("Nil job provided")
	}

	pv := &v1.PersistentVolume{}
	pv.Name = *job.Name

	pvs := v1.PersistentVolumeStatus{
		Message: *job.StatusDescription,
		Reason:  *job.Status,
	}
	pv.Status = pvs

	if *job.Status == structs.JobStatusRunning {
		pv.Annotations = job.Meta
	}

	return pv, nil
}
