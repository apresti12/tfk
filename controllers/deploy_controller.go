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
	"errors"
	"fmt"
	"github.com/go-logr/logr"
	tfkv1beta1 "github.com/tony-mw-tfk/api/v1beta1"
	"gopkg.in/src-d/go-billy.v4/memfs"
	"gopkg.in/src-d/go-git.v4"
	"gopkg.in/src-d/go-git.v4/plumbing"
	"gopkg.in/src-d/go-git.v4/storage/memory"
	core "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"time"
)

var sa core.ServiceAccount

// DeployReconciler reconciles a Deploy object
type DeployReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

type ValidTerraformCommands struct {
	Commands []string
}

type RepoState struct {
	Repo            *git.Repository
	LastKnownCommit plumbing.Hash
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
func ConstructPod(tfDeployment tfkv1beta1.Deploy, command string) (core.Pod, error) {
	// Composite Literals
	name := fmt.Sprintf("terraform-%s", command)

	metaData := metav1.ObjectMeta{
		Labels: map[string]string{
			"terraform": "yes",
		},
		Annotations: make(map[string]string),
		Name:        name,
		Namespace:   tfDeployment.Namespace,
	}

	tfContainer := core.Container{
		Name:       name,
		Image:      tfDeployment.Spec.Terraform.Image,
		Args:       []string{command},
		WorkingDir: tfDeployment.Spec.Source.EntryPoint,
	}

	terraformPodSpec := core.PodSpec{
		ServiceAccountName: tfDeployment.Spec.ServiceAccount,
		Containers:         []core.Container{tfContainer},
	}

	d := core.Pod{
		ObjectMeta: metaData,
		Spec:       terraformPodSpec,
	}

	return d, nil
}

func (r *DeployReconciler) ServiceAccountExists(ctx context.Context, d tfkv1beta1.DeploySpec, req ctrl.Request) bool {
	err := r.Get(ctx, client.ObjectKey{Name: d.ServiceAccount, Namespace: req.Namespace}, &sa)
	if err != nil {
		return false
	} else {
		return true
	}
}

func ConstructServiceAccount(d tfkv1beta1.DeploySpec, req ctrl.Request) core.ServiceAccount {
	return core.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      d.ServiceAccount,
			Namespace: req.Namespace,
		},
	}
}

func (r *DeployReconciler) RunTerraform(deployment tfkv1beta1.Deploy, ctx context.Context, req ctrl.Request, l logr.Logger) (ctrl.Result, error) {

	tfCommands := ValidTerraformCommands{
		Commands: []string{"init", "fmt", "validate", "plan", "apply"},
	}

	var k8sPods core.PodList
	if err := r.List(ctx, &k8sPods, client.InNamespace(req.Namespace)); err != nil {
		l.Error(err, "unable to list children")
		return ctrl.Result{}, err
	}
	for _, v := range k8sPods.Items {
		l.Info("pod exists", v.Name, v.Namespace)
	}

	if len(k8sPods.Items) == 0 {
		l.Info("No pods.")
	}

	//Check for Service Account first
	if !r.ServiceAccountExists(ctx, deployment.Spec, req) {
		s := ConstructServiceAccount(deployment.Spec, req)
		if err := r.Create(ctx, &s); err != nil {
			l.Error(err, "could not create service account")
		}
	}

	//Set Up HashMap
	hm := map[string]bool{
		"init":     true,
		"fmt":      false,
		"validate": false,
		"plan":     false,
		"apply":    false,
	}
	orderedHM := map[int]string{}
	for _, v := range deployment.Spec.Terraform.Commands {
		if _, ok := hm[v]; ok {
			hm[v] = true
		} else {
			l.Error(errors.New("this command is: "), "problematic bro")
		}
	}
	for i, v := range tfCommands.Commands {
		if _, ok := hm[v]; ok {
			orderedHM[i] = v
		}
	}
	for i := 0; i < len(orderedHM); i++ {
		l.Info(orderedHM[i])
		if p, err := ConstructPod(deployment, orderedHM[i]); err != nil {
			l.Error(err, "couldn't construct a full Pod")
		} else {
			err = r.Create(ctx, &p)
			if err != nil {
				l.Error(err, "couldn't run container")
			}
			for {
				err = r.Get(ctx, req.NamespacedName, &p)
				if string(p.Status.Phase) == "Succeeded" {
					break
				} else if string(p.Status.Phase) == "Failed" {
					l.Error(errors.New("pod is failed"), string(p.Status.Phase))
				}
				l.Info(string(p.Status.Phase))
				time.Sleep(time.Second * 5)
			}
		}
	}
	return ctrl.Result{}, nil
}

func InitRepo(repoSpec tfkv1beta1.SourceSpec, l logr.Logger) *RepoState {
	fs := memfs.New()
	storer := memory.NewStorage()
	r, err := git.Clone(storer, fs, &git.CloneOptions{
		URL:           repoSpec.Repo,
		ReferenceName: plumbing.ReferenceName(repoSpec.Branch),
		Depth:         3,
	})
	if err != nil {
		l.Error(err, "trouble cloning repo")
	}
	repoCommits, err := r.CommitObjects()
	if err != nil {
		l.Error(err, "trouble cloning repo")
	}

	latestCommit, err := repoCommits.Next()

	repoState := RepoState{
		Repo:            r,
		LastKnownCommit: latestCommit.Hash,
	}

	return &repoState
}

func (rs *RepoState) WatchRepo(trigger chan bool, l logr.Logger, s tfkv1beta1.SourceSpec) {
	for {
		err := rs.Repo.Fetch(&git.FetchOptions{
			Depth: 3,
		})
		if err != nil {
			l.Error(err, "couldnt fetch")
		}
		repoCommits, err := rs.Repo.CommitObjects()
		if err != nil {
			l.Error(err, "trouble cloning repo")
		}

		latestCommit, err := repoCommits.Next()
		if latestCommit.Hash != rs.LastKnownCommit {
			trigger <- true
		}

		time.Sleep(time.Second * time.Duration(s.RefreshInterval))
	}

}

func (r *DeployReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	l := log.FromContext(ctx)
	var deployment tfkv1beta1.Deploy

	if err := r.Get(ctx, req.NamespacedName, &deployment); err != nil {
		l.Error(err, "Unable to load deployment")
		return ctrl.Result{}, nil
	}
	l.Info("Loaded deployment")

	repoState := InitRepo(deployment.Spec.Source, l)

	// Need to watch for events here
	trigger := make(chan bool)

	//Run in another goroutine
	go repoState.WatchRepo(trigger, l, deployment.Spec.Source)

	// block here til we put true on the channel
	<-trigger
	result, err := r.RunTerraform(deployment, ctx, req, l)
	if err != nil {
		l.Error(err, "could not run terraform")
	}
	return result, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *DeployReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&tfkv1beta1.Deploy{}).
		Complete(r)
}
