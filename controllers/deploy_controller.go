/*
Copyright 2022.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controllers

import (
	"context"
	kapps "k8s.io/api/apps/v1"
	core "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	tfkv1beta1 "github.com/tony-mw-tfk/api/v1beta1"
)

// DeployReconciler reconciles a Deploy object
type DeployReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=tfk.github.com,resources=deploys,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=tfk.github.com,resources=deploys/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=tfk.github.com,resources=deploys/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the Deploy object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.12.2/pkg/reconcile
func ContstructDeployment(tfDeployment tfkv1beta1.Deploy) (kapps.Deployment, error) {
	// Composite Literals
	name := "terraform-deploy"
	metaData := metav1.ObjectMeta{
		Labels: map[string]string{
			"terraform": "yes",
		},
		Annotations: make(map[string]string),
		Name:        name,
		Namespace:   tfDeployment.Namespace,
	}
	replicas := int32(1)
	tfContainer := core.Container{
		Name:  name,
		Image: tfDeployment.Spec.Terraform.Image,
		//TODO Support init, plan, apply, etc
		Args:       []string{"apply"},
		WorkingDir: tfDeployment.Spec.Source.EntryPoint,
	}
	terraformPodSpec := core.PodSpec{
		ServiceAccountName: tfDeployment.Spec.ServiceAccount,
		Containers:         []core.Container{tfContainer},
	}
	terraformTemplate := core.PodTemplateSpec{
		ObjectMeta: metaData,
		Spec:       terraformPodSpec,
	}

	d := kapps.Deployment{
		ObjectMeta: metaData,
		Spec: kapps.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: metaData.Labels,
			},
			Template: terraformTemplate,
		},
	}
	return d, nil
}

func ServiceAccountExists(ctx context.Context, r *DeployReconciler, d tfkv1beta1.DeploySpec, req ctrl.Request) bool {
	sa := core.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{Name: d.ServiceAccount},
	}

	err := r.Get(ctx, client.ObjectKey{
		Namespace: req.Namespace,
	}, &sa)
	if err != nil {
		return false
	} else {
		return true
	}
}

func CreateServiceAccount() {}

func (r *DeployReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	l := log.FromContext(ctx)

	// TODO(user): your logic here
	var deployment tfkv1beta1.Deploy

	if err := r.Get(ctx, req.NamespacedName, &deployment); err != nil {
		l.Error(err, "Unable to load deployment")
		return ctrl.Result{}, nil
	}
	l.Info("Loaded deployment")

	var k8sDeployments kapps.DeploymentList
	if err := r.List(ctx, &k8sDeployments, client.InNamespace(req.Namespace)); err != nil {
		l.Error(err, "unable to list children")
		return ctrl.Result{}, err
	}

	for _, v := range k8sDeployments.Items {
		l.Info("Deployment exists", v.Name)
	}

	if len(k8sDeployments.Items) == 0 {
		l.Info("No deployment items.")
	}
	//Check for Service Account first
	if !ServiceAccountExists(ctx, r, deployment.Spec, req) {
		CreateServiceAccount()
	}
	//Create Deployment
	if d, err := ContstructDeployment(deployment); err != nil {
		l.Error(err, "couldn't construct a full deployment")
	} else {
		err = r.Create(ctx, &d)
		if err != nil {
			l.Error(err, "couldn't create deployment")
		}
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *DeployReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&tfkv1beta1.Deploy{}).
		Complete(r)
}
