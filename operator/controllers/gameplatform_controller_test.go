package controllers

import (
	"context"
	"testing"

	opsv1alpha1 "github.com/Shaohan-He/game-k8s-ops-practice/operator/api/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func testScheme(t *testing.T) *runtime.Scheme {
	t.Helper()
	scheme := runtime.NewScheme()
	if err := clientgoscheme.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	if err := opsv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	return scheme
}

func testPlatform() *opsv1alpha1.GamePlatform {
	enabled := true
	return &opsv1alpha1.GamePlatform{
		TypeMeta:   metav1.TypeMeta{APIVersion: "ops.shaohan.dev/v1alpha1", Kind: "GamePlatform"},
		ObjectMeta: metav1.ObjectMeta{Name: "game-platform", Namespace: "game-ops"},
		Spec: opsv1alpha1.GamePlatformSpec{
			ImageRegistry: "game-k8s-ops-practice",
			ImageTag:      "2.0.0",
			Replicas:      2,
			Ingress:       opsv1alpha1.IngressSpec{Host: "game.local"},
			Monitoring:    opsv1alpha1.MonitoringSpec{Enabled: &enabled},
			Storage:       opsv1alpha1.StorageSpec{MySQL: opsv1alpha1.MySQLStorageSpec{Size: "2Gi"}},
		},
	}
}

func newTestReconciler(t *testing.T, objects ...client.Object) (*GamePlatformReconciler, *opsv1alpha1.GamePlatform) {
	t.Helper()
	scheme := testScheme(t)
	platform := testPlatform()
	allObjects := append([]client.Object{platform}, objects...)
	client := fake.NewClientBuilder().WithScheme(scheme).WithStatusSubresource(platform, &appsv1.Deployment{}).WithObjects(allObjects...).Build()
	return &GamePlatformReconciler{Client: client, Scheme: scheme}, platform
}

func reconcileOnce(t *testing.T, r *GamePlatformReconciler, platform *opsv1alpha1.GamePlatform) {
	t.Helper()
	_, err := r.Reconcile(context.Background(), ctrl.Request{NamespacedName: types.NamespacedName{Name: platform.Name, Namespace: platform.Namespace}})
	if err != nil {
		t.Fatalf("reconcile failed: %v", err)
	}
}

func TestReconcileCreatesFullStackResources(t *testing.T) {
	r, platform := newTestReconciler(t)
	reconcileOnce(t, r, platform)

	for _, name := range []string{"game-app-config", "mysql-init", "prometheus-config", "grafana-dashboard"} {
		var cm corev1.ConfigMap
		if err := r.Get(context.Background(), types.NamespacedName{Name: name, Namespace: platform.Namespace}, &cm); err != nil {
			t.Fatalf("expected ConfigMap %s: %v", name, err)
		}
	}
	for _, name := range []string{"mysql", "redis", "kafka", "game-gateway", "login-service", "match-service", "room-service", "prometheus", "alertmanager", "grafana"} {
		var deploy appsv1.Deployment
		if err := r.Get(context.Background(), types.NamespacedName{Name: name, Namespace: platform.Namespace}, &deploy); err != nil {
			t.Fatalf("expected Deployment %s: %v", name, err)
		}
	}
	var ingress networkingv1.Ingress
	if err := r.Get(context.Background(), types.NamespacedName{Name: "game-gateway", Namespace: platform.Namespace}, &ingress); err != nil {
		t.Fatalf("expected game ingress: %v", err)
	}
}

func TestReconcileUpdatesApplicationImagesFromTag(t *testing.T) {
	r, platform := newTestReconciler(t)
	reconcileOnce(t, r, platform)

	var latest opsv1alpha1.GamePlatform
	if err := r.Get(context.Background(), types.NamespacedName{Name: platform.Name, Namespace: platform.Namespace}, &latest); err != nil {
		t.Fatal(err)
	}
	latest.Spec.ImageTag = "2.1.0"
	if err := r.Update(context.Background(), &latest); err != nil {
		t.Fatal(err)
	}
	reconcileOnce(t, r, &latest)

	var deploy appsv1.Deployment
	if err := r.Get(context.Background(), types.NamespacedName{Name: "login-service", Namespace: platform.Namespace}, &deploy); err != nil {
		t.Fatal(err)
	}
	got := deploy.Spec.Template.Spec.Containers[0].Image
	want := "game-k8s-ops-practice/login-service:2.1.0"
	if got != want {
		t.Fatalf("image = %s, want %s", got, want)
	}
}

func TestReconcileRecreatesMissingDeployment(t *testing.T) {
	r, platform := newTestReconciler(t)
	reconcileOnce(t, r, platform)

	var deploy appsv1.Deployment
	key := types.NamespacedName{Name: "login-service", Namespace: platform.Namespace}
	if err := r.Get(context.Background(), key, &deploy); err != nil {
		t.Fatal(err)
	}
	if err := r.Delete(context.Background(), &deploy); err != nil {
		t.Fatal(err)
	}
	reconcileOnce(t, r, platform)
	if err := r.Get(context.Background(), key, &deploy); err != nil {
		t.Fatalf("expected login-service to be recreated: %v", err)
	}
}

func TestRefreshStatusReportsReadyWhenChildrenAreReady(t *testing.T) {
	r, platform := newTestReconciler(t)
	reconcileOnce(t, r, platform)

	for _, name := range []string{"mysql", "redis", "kafka", "game-gateway", "login-service", "match-service", "room-service", "prometheus", "alertmanager", "grafana"} {
		var deploy appsv1.Deployment
		if err := r.Get(context.Background(), types.NamespacedName{Name: name, Namespace: platform.Namespace}, &deploy); err != nil {
			t.Fatal(err)
		}
		desired := int32(1)
		if deploy.Spec.Replicas != nil {
			desired = *deploy.Spec.Replicas
		}
		deploy.Status.ReadyReplicas = desired
		deploy.Status.UpdatedReplicas = desired
		deploy.Status.ObservedGeneration = deploy.Generation + 10
		if err := r.Status().Update(context.Background(), &deploy); err != nil {
			t.Fatal(err)
		}
	}

	var latest opsv1alpha1.GamePlatform
	if err := r.Get(context.Background(), types.NamespacedName{Name: platform.Name, Namespace: platform.Namespace}, &latest); err != nil {
		t.Fatal(err)
	}
	if err := r.refreshStatus(context.Background(), &latest); err != nil {
		t.Fatal(err)
	}
	if latest.Status.Phase != opsv1alpha1.PhaseReady {
		t.Fatalf("phase = %s, want %s", latest.Status.Phase, opsv1alpha1.PhaseReady)
	}
}
