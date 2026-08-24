package runner

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/dbmove/dbmove/backend/internal/model"
	"github.com/dbmove/dbmove/backend/internal/sse"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// K8sRunner executes workers as Kubernetes Jobs with secrets injected via
// environment variables.
type K8sRunner struct {
	Namespace     string
	WorkerImage   string
	APIURL        string
	InternalToken string
	Resources     K8sResources
	TTLSeconds    int
	Watcher       *Watcher
	client        kubernetes.Interface
}

// K8sResources configures the worker Job resource requests/limits.
type K8sResources struct {
	RequestsCPU    string
	RequestsMemory string
	LimitsCPU      string
	LimitsMemory   string
}

func NewK8sRunner(kubeconfig, namespace, workerImage, apiURL string, res K8sResources, ttl int, w *Watcher) (*K8sRunner, error) {
	var cfg *rest.Config
	var err error
	if kubeconfig != "" {
		cfg, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
	} else {
		cfg, err = rest.InClusterConfig()
	}
	if err != nil {
		// Fall back to default loading rules so local kubectl configs work.
		loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
		cfg, err = clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, &clientcmd.ConfigOverrides{}).ClientConfig()
	}
	if err != nil {
		return nil, fmt.Errorf("build kubernetes config: %w", err)
	}
	clientset, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("create kubernetes client: %w", err)
	}
	return &K8sRunner{
		Namespace:   namespace,
		WorkerImage: workerImage,
		APIURL:      apiURL,
		Resources:   res,
		TTLSeconds:  ttl,
		Watcher:     w,
		client:      clientset,
	}, nil
}

func jobName(taskID uint64) string {
	return fmt.Sprintf("dbmove-migration-%d", taskID)
}

func secretName(taskID uint64) string {
	return fmt.Sprintf("dbmove-migration-%d-secret", taskID)
}

// Start creates the Secret and Job for a migration task.
func (r *K8sRunner) Start(ctx context.Context, task *model.MigrationTask, secrets Secrets) error {
	ns := r.Namespace
	if _, err := r.client.CoreV1().Namespaces().Get(ctx, ns, metav1.GetOptions{}); err != nil {
		if apierrors.IsNotFound(err) {
			if _, cerr := r.client.CoreV1().Namespaces().Create(ctx, &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{Name: ns},
			}, metav1.CreateOptions{}); cerr != nil {
				return fmt.Errorf("create namespace %s: %w", ns, cerr)
			}
		} else {
			return fmt.Errorf("get namespace %s: %w", ns, err)
		}
	}

	sec := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName(task.ID),
			Namespace: ns,
			Labels:    map[string]string{"app": "dbmove", "task-id": fmt.Sprintf("%d", task.ID)},
		},
		StringData: map[string]string{
			"source-password": secrets.SourcePassword,
			"target-password": secrets.TargetPassword,
		},
	}
	if _, err := r.client.CoreV1().Secrets(ns).Create(ctx, sec, metav1.CreateOptions{}); err != nil && !apierrors.IsAlreadyExists(err) {
		return fmt.Errorf("create secret: %w", err)
	}

	backoff := int32(0)
	ttl := int32(r.TTLSeconds)
	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      jobName(task.ID),
			Namespace: ns,
			Labels:    map[string]string{"app": "dbmove", "task-id": fmt.Sprintf("%d", task.ID)},
		},
		Spec: batchv1.JobSpec{
			BackoffLimit:            &backoff,
			TTLSecondsAfterFinished: &ttl,
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app": "dbmove", "task-id": fmt.Sprintf("%d", task.ID)},
				},
				Spec: corev1.PodSpec{
					RestartPolicy: corev1.RestartPolicyNever,
					Containers: []corev1.Container{
						{
							Name:  "migration-worker",
							Image: r.WorkerImage,
							Env: []corev1.EnvVar{
								{Name: "TASK_ID", Value: fmt.Sprintf("%d", task.ID)},
								{Name: "DBMOVE_API", Value: r.APIURL},
								{Name: "DBMOVE_INTERNAL_TOKEN", Value: r.InternalToken},
								{
									Name: "DBMOVE_SOURCE_PASSWORD",
									ValueFrom: &corev1.EnvVarSource{
										SecretKeyRef: &corev1.SecretKeySelector{
											LocalObjectReference: corev1.LocalObjectReference{Name: secretName(task.ID)},
											Key:                  "source-password",
										},
									},
								},
								{
									Name: "DBMOVE_TARGET_PASSWORD",
									ValueFrom: &corev1.EnvVarSource{
										SecretKeyRef: &corev1.SecretKeySelector{
											LocalObjectReference: corev1.LocalObjectReference{Name: secretName(task.ID)},
											Key:                  "target-password",
										},
									},
								},
							},
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse(r.Resources.RequestsCPU),
									corev1.ResourceMemory: resource.MustParse(r.Resources.RequestsMemory),
								},
								Limits: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse(r.Resources.LimitsCPU),
									corev1.ResourceMemory: resource.MustParse(r.Resources.LimitsMemory),
								},
							},
						},
					},
				},
			},
		},
	}
	if _, err := r.client.BatchV1().Jobs(ns).Create(ctx, job, metav1.CreateOptions{}); err != nil && !apierrors.IsAlreadyExists(err) {
		return fmt.Errorf("create job: %w", err)
	}
	go r.watch(task.ID)
	return nil
}

// Cancel deletes the Job and Secret for a task.
func (r *K8sRunner) Cancel(ctx context.Context, taskID uint64) error {
	prop := metav1.DeletePropagationBackground
	_ = r.client.BatchV1().Jobs(r.Namespace).Delete(ctx, jobName(taskID), metav1.DeleteOptions{PropagationPolicy: &prop})
	_ = r.client.CoreV1().Secrets(r.Namespace).Delete(ctx, secretName(taskID), metav1.DeleteOptions{})
	return nil
}

// Cleanup removes Job and Secret resources.
func (r *K8sRunner) Cleanup(ctx context.Context, taskID uint64) error {
	return r.Cancel(ctx, taskID)
}

func (r *K8sRunner) watch(taskID uint64) {
	ctx, cancel := context.WithTimeout(context.Background(), 48*time.Hour)
	defer cancel()
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			job, err := r.client.BatchV1().Jobs(r.Namespace).Get(ctx, jobName(taskID), metav1.GetOptions{})
			if err != nil {
				if apierrors.IsNotFound(err) {
					task, gerr := r.Watcher.Repo.GetTask(ctx, taskID)
					if gerr == nil && !IsTerminal(task.Status) {
						_ = r.Watcher.MarkFailedIfStillActive(ctx, taskID, "migration job was removed")
					}
					_ = r.Cleanup(ctx, taskID)
					return
				}
				continue
			}
			if job.Status.Succeeded > 0 {
				task, gerr := r.Watcher.Repo.GetTask(ctx, taskID)
				if gerr == nil && !IsTerminal(task.Status) {
					r.markSuccess(ctx, taskID)
				}
				_ = r.Cleanup(ctx, taskID) // remove Secret and completed Job
				return
			}
			if job.Status.Failed > 0 {
				reason := "migration job failed"
				if tail := r.podLogTail(ctx, taskID); tail != "" {
					reason = "migration job failed: " + tail
				}
				_ = r.Watcher.MarkFailedIfStillActive(ctx, taskID, reason)
				_ = r.Cleanup(ctx, taskID) // remove Secret and failed Job
				return
			}
		}
	}
}

func (r *K8sRunner) markSuccess(ctx context.Context, taskID uint64) {
	now := time.Now()
	_ = r.Watcher.Repo.UpdateTaskFields(ctx, taskID, map[string]any{
		"status":      model.TaskStatusSuccess,
		"finished_at": now,
	})
	if r.Watcher.Hub != nil {
		r.Watcher.Hub.Publish(taskID, sse.Event{Type: "status", Data: map[string]any{"status": model.TaskStatusSuccess}})
	}
}

func (r *K8sRunner) podLogTail(ctx context.Context, taskID uint64) string {
	pods, err := r.client.CoreV1().Pods(r.Namespace).List(ctx, metav1.ListOptions{
		LabelSelector: "app=dbmove,task-id=" + fmt.Sprintf("%d", taskID),
	})
	if err != nil || len(pods.Items) == 0 {
		return ""
	}
	pod := pods.Items[len(pods.Items)-1]
	req := r.client.CoreV1().Pods(r.Namespace).GetLogs(pod.Name, &corev1.PodLogOptions{TailLines: int64ptr(30)})
	data, err := req.DoRaw(ctx)
	if err != nil {
		return ""
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) > 5 {
		lines = lines[len(lines)-5:]
	}
	return strings.Join(lines, " | ")
}

func int64ptr(v int64) *int64 { return &v }

var _ Runner = (*K8sRunner)(nil)
var _ Cleanup = (*K8sRunner)(nil)
var _ Cleanup = (*DockerRunner)(nil)
