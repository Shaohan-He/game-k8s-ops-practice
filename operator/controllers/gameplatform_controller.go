package controllers

import (
	"context"
	"fmt"
	"reflect"

	opsv1alpha1 "github.com/Shaohan-He/game-k8s-ops-practice/operator/api/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	defaultRegistry           = "game-k8s-ops-practice"
	defaultTag                = "2.0.0"
	defaultReplicas     int32 = 2
	defaultIngressHost        = "game.local"
	defaultIngressClass       = "nginx"
	defaultMySQLSize          = "2Gi"
	partOf                    = "game-k8s-ops-practice"
	appConfigName             = "game-app-config"
	appSecretName             = "game-app-secret"
)

type GamePlatformReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

type appService struct {
	Name string
	Port int32
}

var appServices = []appService{
	{Name: "game-gateway", Port: 8000},
	{Name: "login-service", Port: 8001},
	{Name: "match-service", Port: 8002},
	{Name: "room-service", Port: 8003},
}

func (r *GamePlatformReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	var platform opsv1alpha1.GamePlatform
	if err := r.Get(ctx, req.NamespacedName, &platform); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	if err := r.ensureAll(ctx, &platform); err != nil {
		logger.Error(err, "failed to reconcile GamePlatform")
		_ = r.updateStatus(ctx, &platform, opsv1alpha1.PhaseDegraded, err.Error())
		return ctrl.Result{}, err
	}

	if err := r.refreshStatus(ctx, &platform); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *GamePlatformReconciler) ensureAll(ctx context.Context, gp *opsv1alpha1.GamePlatform) error {
	steps := []func(context.Context, *opsv1alpha1.GamePlatform) error{
		r.ensureConfig,
		r.ensureInfra,
		r.ensureApps,
		r.ensureIngress,
	}
	if monitoringEnabled(gp) {
		steps = append(steps, r.ensureMonitoring)
	}

	for _, step := range steps {
		if err := step(ctx, gp); err != nil {
			return err
		}
	}
	return nil
}

func (r *GamePlatformReconciler) ensureConfig(ctx context.Context, gp *opsv1alpha1.GamePlatform) error {
	config := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: appConfigName, Namespace: gp.Namespace}}
	if err := r.reconcileObject(ctx, gp, config, func() error {
		config.Labels = commonLabels("game-config")
		config.Data = map[string]string{
			"REDIS_URL":               "redis://redis:6379/0",
			"MYSQL_HOST":              "mysql",
			"MYSQL_PORT":              "3306",
			"MYSQL_DATABASE":          "game_ops",
			"MYSQL_USER":              "game",
			"KAFKA_BOOTSTRAP_SERVERS": "kafka:9092",
			"LOGIN_SERVICE_URL":       "http://login-service:8001",
			"MATCH_SERVICE_URL":       "http://match-service:8002",
			"ROOM_SERVICE_URL":        "http://room-service:8003",
			"LOG_LEVEL":               "INFO",
		}
		return nil
	}); err != nil {
		return err
	}

	secret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: appSecretName, Namespace: gp.Namespace}}
	if err := r.reconcileObject(ctx, gp, secret, func() error {
		secret.Labels = commonLabels("game-secret")
		secret.Type = corev1.SecretTypeOpaque
		secret.Data = map[string][]byte{
			"MYSQL_PASSWORD":         []byte("game_password"),
			"MYSQL_ROOT_PASSWORD":    []byte("root_password"),
			"GRAFANA_ADMIN_USER":     []byte("admin"),
			"GRAFANA_ADMIN_PASSWORD": []byte("admin"),
		}
		return nil
	}); err != nil {
		return err
	}

	configMaps := map[string]map[string]string{
		"mysql-init":           {"init.sql": mysqlInitSQL},
		"prometheus-config":    {"prometheus.yml": prometheusConfig, "alerts.yml": alertsConfig},
		"alertmanager-config":  {"alertmanager.yml": alertmanagerConfig},
		"grafana-provisioning": {"datasource.yml": grafanaDatasourceConfig, "dashboard.yml": grafanaDashboardProviderConfig},
		"grafana-dashboard":    {"game-services-overview.json": grafanaDashboardConfig},
	}
	for name, data := range configMaps {
		cm := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: gp.Namespace}}
		if err := r.reconcileObject(ctx, gp, cm, func() error {
			cm.Labels = commonLabels(name)
			cm.Data = data
			return nil
		}); err != nil {
			return err
		}
	}

	return nil
}

func (r *GamePlatformReconciler) ensureInfra(ctx context.Context, gp *opsv1alpha1.GamePlatform) error {
	if err := r.ensureMySQL(ctx, gp); err != nil {
		return err
	}
	if err := r.ensureRedis(ctx, gp); err != nil {
		return err
	}
	return r.ensureKafka(ctx, gp)
}

func (r *GamePlatformReconciler) ensureMySQL(ctx context.Context, gp *opsv1alpha1.GamePlatform) error {
	pvc := &corev1.PersistentVolumeClaim{ObjectMeta: metav1.ObjectMeta{Name: "mysql-data", Namespace: gp.Namespace}}
	if err := r.reconcileObject(ctx, gp, pvc, func() error {
		pvc.Labels = commonLabels("mysql")
		pvc.Spec.AccessModes = []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce}
		pvc.Spec.Resources.Requests = corev1.ResourceList{corev1.ResourceStorage: resource.MustParse(mysqlStorageSize(gp))}
		return nil
	}); err != nil {
		return err
	}

	deploy := &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: "mysql", Namespace: gp.Namespace}}
	if err := r.reconcileObject(ctx, gp, deploy, func() error {
		deploy.Labels = commonLabels("mysql")
		deploy.Spec.Replicas = int32Ptr(1)
		deploy.Spec.Selector = &metav1.LabelSelector{MatchLabels: appLabels("mysql")}
		deploy.Spec.Strategy.Type = appsv1.RecreateDeploymentStrategyType
		deploy.Spec.Template.ObjectMeta.Labels = appLabels("mysql")
		deploy.Spec.Template.Spec.Containers = []corev1.Container{{
			Name:  "mysql",
			Image: "mysql:8.4.5",
			Ports: []corev1.ContainerPort{{Name: "mysql", ContainerPort: 3306}},
			Env: []corev1.EnvVar{
				configEnv("MYSQL_DATABASE", appConfigName, "MYSQL_DATABASE"),
				configEnv("MYSQL_USER", appConfigName, "MYSQL_USER"),
				secretEnv("MYSQL_PASSWORD", appSecretName, "MYSQL_PASSWORD"),
				secretEnv("MYSQL_ROOT_PASSWORD", appSecretName, "MYSQL_ROOT_PASSWORD"),
			},
			VolumeMounts: []corev1.VolumeMount{{Name: "data", MountPath: "/var/lib/mysql"}, {Name: "init", MountPath: "/docker-entrypoint-initdb.d", ReadOnly: true}},
			ReadinessProbe: &corev1.Probe{
				ProbeHandler:        corev1.ProbeHandler{Exec: &corev1.ExecAction{Command: []string{"sh", "-c", "mysqladmin ping -h 127.0.0.1 -uroot -p$MYSQL_ROOT_PASSWORD"}}},
				InitialDelaySeconds: 20,
				PeriodSeconds:       10,
			},
			Resources: resources("200m", "512Mi", "1", "1Gi"),
		}}
		deploy.Spec.Template.Spec.Volumes = []corev1.Volume{
			{Name: "data", VolumeSource: corev1.VolumeSource{PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{ClaimName: "mysql-data"}}},
			{Name: "init", VolumeSource: corev1.VolumeSource{ConfigMap: &corev1.ConfigMapVolumeSource{LocalObjectReference: corev1.LocalObjectReference{Name: "mysql-init"}}}},
		}
		return nil
	}); err != nil {
		return err
	}

	return r.ensureService(ctx, gp, "mysql", []corev1.ServicePort{{Name: "mysql", Port: 3306}})
}

func (r *GamePlatformReconciler) ensureRedis(ctx context.Context, gp *opsv1alpha1.GamePlatform) error {
	deploy := &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: "redis", Namespace: gp.Namespace}}
	if err := r.reconcileObject(ctx, gp, deploy, func() error {
		deploy.Labels = commonLabels("redis")
		deploy.Spec.Replicas = int32Ptr(1)
		deploy.Spec.Selector = &metav1.LabelSelector{MatchLabels: appLabels("redis")}
		deploy.Spec.Template.ObjectMeta.Labels = appLabels("redis")
		deploy.Spec.Template.Spec.Containers = []corev1.Container{{
			Name:           "redis",
			Image:          "redis:7.4.2-alpine",
			Args:           []string{"redis-server", "--appendonly", "yes"},
			Ports:          []corev1.ContainerPort{{Name: "redis", ContainerPort: 6379}},
			ReadinessProbe: &corev1.Probe{ProbeHandler: corev1.ProbeHandler{Exec: &corev1.ExecAction{Command: []string{"redis-cli", "ping"}}}, InitialDelaySeconds: 5, PeriodSeconds: 5},
			Resources:      resources("50m", "64Mi", "250m", "256Mi"),
		}}
		return nil
	}); err != nil {
		return err
	}

	return r.ensureService(ctx, gp, "redis", []corev1.ServicePort{{Name: "redis", Port: 6379}})
}

func (r *GamePlatformReconciler) ensureKafka(ctx context.Context, gp *opsv1alpha1.GamePlatform) error {
	deploy := &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: "kafka", Namespace: gp.Namespace}}
	if err := r.reconcileObject(ctx, gp, deploy, func() error {
		deploy.Labels = commonLabels("kafka")
		deploy.Spec.Replicas = int32Ptr(1)
		deploy.Spec.Selector = &metav1.LabelSelector{MatchLabels: appLabels("kafka")}
		deploy.Spec.Template.ObjectMeta.Labels = appLabels("kafka")
		deploy.Spec.Template.Spec.Containers = []corev1.Container{{
			Name:  "kafka",
			Image: "apache/kafka:3.9.1",
			Ports: []corev1.ContainerPort{{Name: "broker", ContainerPort: 9092}, {Name: "controller", ContainerPort: 9093}},
			Env: []corev1.EnvVar{
				{Name: "KAFKA_NODE_ID", Value: "1"},
				{Name: "KAFKA_PROCESS_ROLES", Value: "broker,controller"},
				{Name: "KAFKA_CONTROLLER_QUORUM_VOTERS", Value: "1@kafka:9093"},
				{Name: "KAFKA_LISTENERS", Value: "PLAINTEXT://:9092,CONTROLLER://:9093"},
				{Name: "KAFKA_ADVERTISED_LISTENERS", Value: "PLAINTEXT://kafka:9092"},
				{Name: "KAFKA_LISTENER_SECURITY_PROTOCOL_MAP", Value: "CONTROLLER:PLAINTEXT,PLAINTEXT:PLAINTEXT"},
				{Name: "KAFKA_CONTROLLER_LISTENER_NAMES", Value: "CONTROLLER"},
				{Name: "KAFKA_INTER_BROKER_LISTENER_NAME", Value: "PLAINTEXT"},
				{Name: "KAFKA_OFFSETS_TOPIC_REPLICATION_FACTOR", Value: "1"},
				{Name: "KAFKA_AUTO_CREATE_TOPICS_ENABLE", Value: "true"},
			},
			ReadinessProbe: &corev1.Probe{ProbeHandler: corev1.ProbeHandler{TCPSocket: &corev1.TCPSocketAction{Port: intstr.FromString("broker")}}, InitialDelaySeconds: 15, PeriodSeconds: 10},
			Resources:      resources("200m", "512Mi", "1", "1Gi"),
		}}
		return nil
	}); err != nil {
		return err
	}

	return r.ensureService(ctx, gp, "kafka", []corev1.ServicePort{{Name: "broker", Port: 9092}, {Name: "controller", Port: 9093}})
}
func (r *GamePlatformReconciler) ensureApps(ctx context.Context, gp *opsv1alpha1.GamePlatform) error {
	for _, svc := range appServices {
		if err := r.ensureAppDeployment(ctx, gp, svc); err != nil {
			return err
		}
		if err := r.ensureService(ctx, gp, svc.Name, []corev1.ServicePort{{Name: "http", Port: svc.Port, TargetPort: intstr.FromString("http")}}); err != nil {
			return err
		}
	}
	return nil
}

func (r *GamePlatformReconciler) ensureAppDeployment(ctx context.Context, gp *opsv1alpha1.GamePlatform, svc appService) error {
	deploy := &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: svc.Name, Namespace: gp.Namespace}}
	return r.reconcileObject(ctx, gp, deploy, func() error {
		deploy.Labels = commonLabels(svc.Name)
		deploy.Spec.Replicas = int32Ptr(replicas(gp))
		deploy.Spec.RevisionHistoryLimit = int32Ptr(5)
		deploy.Spec.Strategy.Type = appsv1.RollingUpdateDeploymentStrategyType
		deploy.Spec.Strategy.RollingUpdate = &appsv1.RollingUpdateDeployment{MaxUnavailable: &intstr.IntOrString{Type: intstr.Int, IntVal: 0}, MaxSurge: &intstr.IntOrString{Type: intstr.Int, IntVal: 1}}
		deploy.Spec.Selector = &metav1.LabelSelector{MatchLabels: appLabels(svc.Name)}
		deploy.Spec.Template.ObjectMeta.Labels = appLabels(svc.Name)
		deploy.Spec.Template.ObjectMeta.Annotations = map[string]string{"prometheus.io/scrape": "true", "prometheus.io/port": fmt.Sprintf("%d", svc.Port), "prometheus.io/path": "/metrics"}
		deploy.Spec.Template.Spec.SecurityContext = &corev1.PodSecurityContext{SeccompProfile: &corev1.SeccompProfile{Type: corev1.SeccompProfileTypeRuntimeDefault}}
		deploy.Spec.Template.Spec.Containers = []corev1.Container{{
			Name:            svc.Name,
			Image:           fmt.Sprintf("%s/%s:%s", imageRegistry(gp), svc.Name, imageTag(gp)),
			ImagePullPolicy: corev1.PullIfNotPresent,
			Ports:           []corev1.ContainerPort{{Name: "http", ContainerPort: svc.Port}},
			EnvFrom: []corev1.EnvFromSource{
				{ConfigMapRef: &corev1.ConfigMapEnvSource{LocalObjectReference: corev1.LocalObjectReference{Name: appConfigName}}},
				{SecretRef: &corev1.SecretEnvSource{LocalObjectReference: corev1.LocalObjectReference{Name: appSecretName}}},
			},
			Env:             []corev1.EnvVar{{Name: "SERVICE_NAME", Value: svc.Name}, {Name: "SERVICE_PORT", Value: fmt.Sprintf("%d", svc.Port)}},
			Resources:       resources("100m", "128Mi", "500m", "256Mi"),
			ReadinessProbe:  httpProbe("/health", 10, 10),
			LivenessProbe:   httpProbe("/health", 20, 10),
			SecurityContext: &corev1.SecurityContext{AllowPrivilegeEscalation: boolPtr(false), ReadOnlyRootFilesystem: boolPtr(true), RunAsNonRoot: boolPtr(true), RunAsUser: int64Ptr(10001), Capabilities: &corev1.Capabilities{Drop: []corev1.Capability{"ALL"}}},
		}}
		return nil
	})
}

func (r *GamePlatformReconciler) ensureMonitoring(ctx context.Context, gp *opsv1alpha1.GamePlatform) error {
	if err := r.ensurePrometheus(ctx, gp); err != nil {
		return err
	}
	if err := r.ensureAlertmanager(ctx, gp); err != nil {
		return err
	}
	return r.ensureGrafana(ctx, gp)
}

func (r *GamePlatformReconciler) ensurePrometheus(ctx context.Context, gp *opsv1alpha1.GamePlatform) error {
	deploy := &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: "prometheus", Namespace: gp.Namespace}}
	if err := r.reconcileObject(ctx, gp, deploy, func() error {
		deploy.Labels = commonLabels("prometheus")
		deploy.Spec.Replicas = int32Ptr(1)
		deploy.Spec.Selector = &metav1.LabelSelector{MatchLabels: appLabels("prometheus")}
		deploy.Spec.Template.ObjectMeta.Labels = appLabels("prometheus")
		deploy.Spec.Template.Spec.Containers = []corev1.Container{{Name: "prometheus", Image: "prom/prometheus:v3.3.0", Args: []string{"--config.file=/etc/prometheus/prometheus.yml", "--storage.tsdb.path=/prometheus"}, Ports: []corev1.ContainerPort{{Name: "http", ContainerPort: 9090}}, VolumeMounts: []corev1.VolumeMount{{Name: "config", MountPath: "/etc/prometheus", ReadOnly: true}}, Resources: resources("100m", "256Mi", "500m", "512Mi")}}
		deploy.Spec.Template.Spec.Volumes = []corev1.Volume{{Name: "config", VolumeSource: corev1.VolumeSource{ConfigMap: &corev1.ConfigMapVolumeSource{LocalObjectReference: corev1.LocalObjectReference{Name: "prometheus-config"}}}}}
		return nil
	}); err != nil {
		return err
	}
	return r.ensureService(ctx, gp, "prometheus", []corev1.ServicePort{{Name: "http", Port: 9090}})
}

func (r *GamePlatformReconciler) ensureAlertmanager(ctx context.Context, gp *opsv1alpha1.GamePlatform) error {
	deploy := &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: "alertmanager", Namespace: gp.Namespace}}
	if err := r.reconcileObject(ctx, gp, deploy, func() error {
		deploy.Labels = commonLabels("alertmanager")
		deploy.Spec.Replicas = int32Ptr(1)
		deploy.Spec.Selector = &metav1.LabelSelector{MatchLabels: appLabels("alertmanager")}
		deploy.Spec.Template.ObjectMeta.Labels = appLabels("alertmanager")
		deploy.Spec.Template.Spec.Containers = []corev1.Container{{Name: "alertmanager", Image: "prom/alertmanager:v0.28.1", Args: []string{"--config.file=/etc/alertmanager/alertmanager.yml"}, Ports: []corev1.ContainerPort{{Name: "http", ContainerPort: 9093}}, VolumeMounts: []corev1.VolumeMount{{Name: "config", MountPath: "/etc/alertmanager", ReadOnly: true}}}}
		deploy.Spec.Template.Spec.Volumes = []corev1.Volume{{Name: "config", VolumeSource: corev1.VolumeSource{ConfigMap: &corev1.ConfigMapVolumeSource{LocalObjectReference: corev1.LocalObjectReference{Name: "alertmanager-config"}}}}}
		return nil
	}); err != nil {
		return err
	}
	return r.ensureService(ctx, gp, "alertmanager", []corev1.ServicePort{{Name: "http", Port: 9093}})
}

func (r *GamePlatformReconciler) ensureGrafana(ctx context.Context, gp *opsv1alpha1.GamePlatform) error {
	deploy := &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: "grafana", Namespace: gp.Namespace}}
	if err := r.reconcileObject(ctx, gp, deploy, func() error {
		deploy.Labels = commonLabels("grafana")
		deploy.Spec.Replicas = int32Ptr(1)
		deploy.Spec.Selector = &metav1.LabelSelector{MatchLabels: appLabels("grafana")}
		deploy.Spec.Template.ObjectMeta.Labels = appLabels("grafana")
		deploy.Spec.Template.Spec.Containers = []corev1.Container{{
			Name:  "grafana",
			Image: "grafana/grafana:11.6.1",
			Ports: []corev1.ContainerPort{{Name: "http", ContainerPort: 3000}},
			Env:   []corev1.EnvVar{secretEnv("GF_SECURITY_ADMIN_USER", appSecretName, "GRAFANA_ADMIN_USER"), secretEnv("GF_SECURITY_ADMIN_PASSWORD", appSecretName, "GRAFANA_ADMIN_PASSWORD"), {Name: "GF_USERS_ALLOW_SIGN_UP", Value: "false"}},
			VolumeMounts: []corev1.VolumeMount{
				{Name: "provisioning", MountPath: "/etc/grafana/provisioning/datasources/prometheus.yml", SubPath: "datasource.yml", ReadOnly: true},
				{Name: "provisioning", MountPath: "/etc/grafana/provisioning/dashboards/dashboard.yml", SubPath: "dashboard.yml", ReadOnly: true},
				{Name: "dashboard", MountPath: "/var/lib/grafana/dashboards/game-services-overview.json", SubPath: "game-services-overview.json", ReadOnly: true},
			},
		}}
		deploy.Spec.Template.Spec.Volumes = []corev1.Volume{{Name: "provisioning", VolumeSource: corev1.VolumeSource{ConfigMap: &corev1.ConfigMapVolumeSource{LocalObjectReference: corev1.LocalObjectReference{Name: "grafana-provisioning"}}}}, {Name: "dashboard", VolumeSource: corev1.VolumeSource{ConfigMap: &corev1.ConfigMapVolumeSource{LocalObjectReference: corev1.LocalObjectReference{Name: "grafana-dashboard"}}}}}
		return nil
	}); err != nil {
		return err
	}
	return r.ensureService(ctx, gp, "grafana", []corev1.ServicePort{{Name: "http", Port: 3000}})
}

func (r *GamePlatformReconciler) ensureIngress(ctx context.Context, gp *opsv1alpha1.GamePlatform) error {
	ingressClass := ingressClassName(gp)
	ingress := &networkingv1.Ingress{ObjectMeta: metav1.ObjectMeta{Name: "game-gateway", Namespace: gp.Namespace}}
	return r.reconcileObject(ctx, gp, ingress, func() error {
		ingress.Labels = commonLabels("game-gateway")
		ingress.Annotations = map[string]string{
			"nginx.ingress.kubernetes.io/proxy-connect-timeout": "3",
			"nginx.ingress.kubernetes.io/proxy-read-timeout":    "10",
		}
		ingress.Spec.IngressClassName = &ingressClass
		ingress.Spec.Rules = []networkingv1.IngressRule{
			{
				Host: ingressHost(gp),
				IngressRuleValue: networkingv1.IngressRuleValue{
					HTTP: &networkingv1.HTTPIngressRuleValue{
						Paths: []networkingv1.HTTPIngressPath{
							{
								Path:     "/",
								PathType: pathTypePtr(networkingv1.PathTypePrefix),
								Backend: networkingv1.IngressBackend{
									Service: &networkingv1.IngressServiceBackend{
										Name: "game-gateway",
										Port: networkingv1.ServiceBackendPort{Number: 8000},
									},
								},
							},
						},
					},
				},
			},
		}
		return nil
	})
}
func (r *GamePlatformReconciler) ensureService(ctx context.Context, gp *opsv1alpha1.GamePlatform, name string, ports []corev1.ServicePort) error {
	service := &corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: gp.Namespace}}
	return r.reconcileObject(ctx, gp, service, func() error {
		service.Labels = commonLabels(name)
		service.Spec.Selector = appLabels(name)
		service.Spec.Ports = ports
		return nil
	})
}

func (r *GamePlatformReconciler) reconcileObject(ctx context.Context, gp *opsv1alpha1.GamePlatform, obj client.Object, mutate func() error) error {
	_, err := controllerutil.CreateOrUpdate(ctx, r.Client, obj, func() error {
		if err := controllerutil.SetControllerReference(gp, obj, r.Scheme); err != nil {
			return err
		}
		return mutate()
	})
	return err
}

func (r *GamePlatformReconciler) refreshStatus(ctx context.Context, gp *opsv1alpha1.GamePlatform) error {
	statuses := make([]opsv1alpha1.ServiceStatus, 0, len(appServices))
	appsReady := true
	for _, svc := range appServices {
		var deploy appsv1.Deployment
		if err := r.Get(ctx, types.NamespacedName{Name: svc.Name, Namespace: gp.Namespace}, &deploy); err != nil {
			statuses = append(statuses, opsv1alpha1.ServiceStatus{Name: svc.Name, Desired: replicas(gp), RolloutState: "Missing"})
			appsReady = false
			continue
		}
		image := ""
		if len(deploy.Spec.Template.Spec.Containers) > 0 {
			image = deploy.Spec.Template.Spec.Containers[0].Image
		}
		rolloutState := "Progressing"
		desired := replicas(gp)
		if deploy.Status.ReadyReplicas >= desired && deploy.Status.UpdatedReplicas >= desired {
			rolloutState = "Ready"
		} else {
			appsReady = false
		}
		statuses = append(statuses, opsv1alpha1.ServiceStatus{Name: svc.Name, Desired: desired, Ready: deploy.Status.ReadyReplicas, Image: image, RolloutState: rolloutState})
	}

	infraReady := r.deploymentsReady(ctx, gp.Namespace, []string{"mysql", "redis", "kafka"})
	monitoringReady := true
	if monitoringEnabled(gp) {
		monitoringReady = r.deploymentsReady(ctx, gp.Namespace, []string{"prometheus", "alertmanager", "grafana"})
	}
	ingressReady := r.objectExists(ctx, &networkingv1.Ingress{}, types.NamespacedName{Name: "game-gateway", Namespace: gp.Namespace})

	phase := opsv1alpha1.PhaseProgressing
	if appsReady && infraReady && monitoringReady && ingressReady {
		phase = opsv1alpha1.PhaseReady
	}

	next := gp.Status.DeepCopy()
	next.Phase = phase
	next.ServiceStatuses = statuses
	next.ObservedGeneration = gp.Generation
	setCondition(&next.Conditions, "AppsReady", appsReady, gp.Generation, "RolloutStatus", conditionMessage(appsReady, "all game services are ready", "one or more game services are still rolling out"))
	setCondition(&next.Conditions, "InfraReady", infraReady, gp.Generation, "RolloutStatus", conditionMessage(infraReady, "Redis, MySQL, and Kafka are ready", "one or more infrastructure deployments are not ready"))
	setCondition(&next.Conditions, "MonitoringReady", monitoringReady, gp.Generation, "RolloutStatus", conditionMessage(monitoringReady, "monitoring stack is ready", "one or more monitoring deployments are not ready"))
	setCondition(&next.Conditions, "IngressReady", ingressReady, gp.Generation, "ResourceExists", conditionMessage(ingressReady, "game ingress exists", "game ingress is missing"))
	setCondition(&next.Conditions, "ReconcileSucceeded", true, gp.Generation, "Reconciled", "desired resources were reconciled")

	if reflect.DeepEqual(gp.Status, *next) {
		return nil
	}
	gp.Status = *next
	return r.Status().Update(ctx, gp)
}

func (r *GamePlatformReconciler) updateStatus(ctx context.Context, gp *opsv1alpha1.GamePlatform, phase, message string) error {
	next := gp.Status.DeepCopy()
	next.Phase = phase
	next.ObservedGeneration = gp.Generation
	setCondition(&next.Conditions, "ReconcileSucceeded", false, gp.Generation, "ReconcileError", message)
	gp.Status = *next
	return r.Status().Update(ctx, gp)
}

func (r *GamePlatformReconciler) deploymentsReady(ctx context.Context, namespace string, names []string) bool {
	for _, name := range names {
		var deploy appsv1.Deployment
		if err := r.Get(ctx, types.NamespacedName{Name: name, Namespace: namespace}, &deploy); err != nil {
			return false
		}
		desired := int32(1)
		if deploy.Spec.Replicas != nil {
			desired = *deploy.Spec.Replicas
		}
		if deploy.Status.ReadyReplicas < desired || deploy.Status.UpdatedReplicas < desired {
			return false
		}
	}
	return true
}

func (r *GamePlatformReconciler) objectExists(ctx context.Context, obj client.Object, key types.NamespacedName) bool {
	return r.Get(ctx, key, obj) == nil
}

func (r *GamePlatformReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&opsv1alpha1.GamePlatform{}).
		Owns(&appsv1.Deployment{}).
		Owns(&corev1.Service{}).
		Owns(&corev1.ConfigMap{}).
		Owns(&corev1.Secret{}).
		Owns(&corev1.PersistentVolumeClaim{}).
		Owns(&networkingv1.Ingress{}).
		Complete(r)
}

func imageRegistry(gp *opsv1alpha1.GamePlatform) string {
	if gp.Spec.ImageRegistry != "" {
		return gp.Spec.ImageRegistry
	}
	return defaultRegistry
}

func imageTag(gp *opsv1alpha1.GamePlatform) string {
	if gp.Spec.ImageTag != "" {
		return gp.Spec.ImageTag
	}
	return defaultTag
}

func replicas(gp *opsv1alpha1.GamePlatform) int32 {
	if gp.Spec.Replicas > 0 {
		return gp.Spec.Replicas
	}
	return defaultReplicas
}

func ingressHost(gp *opsv1alpha1.GamePlatform) string {
	if gp.Spec.Ingress.Host != "" {
		return gp.Spec.Ingress.Host
	}
	return defaultIngressHost
}

func ingressClassName(gp *opsv1alpha1.GamePlatform) string {
	if gp.Spec.Ingress.IngressClassName != "" {
		return gp.Spec.Ingress.IngressClassName
	}
	return defaultIngressClass
}

func monitoringEnabled(gp *opsv1alpha1.GamePlatform) bool {
	if gp.Spec.Monitoring.Enabled == nil {
		return true
	}
	return *gp.Spec.Monitoring.Enabled
}

func mysqlStorageSize(gp *opsv1alpha1.GamePlatform) string {
	if gp.Spec.Storage.MySQL.Size != "" {
		return gp.Spec.Storage.MySQL.Size
	}
	return defaultMySQLSize
}

func commonLabels(name string) map[string]string {
	labels := appLabels(name)
	labels["app.kubernetes.io/part-of"] = partOf
	labels["app.kubernetes.io/managed-by"] = "game-platform-operator"
	return labels
}

func appLabels(name string) map[string]string {
	return map[string]string{"app": name}
}

func resources(cpuRequest, memoryRequest, cpuLimit, memoryLimit string) corev1.ResourceRequirements {
	return corev1.ResourceRequirements{
		Requests: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse(cpuRequest), corev1.ResourceMemory: resource.MustParse(memoryRequest)},
		Limits:   corev1.ResourceList{corev1.ResourceCPU: resource.MustParse(cpuLimit), corev1.ResourceMemory: resource.MustParse(memoryLimit)},
	}
}

func httpProbe(path string, initialDelay, period int32) *corev1.Probe {
	return &corev1.Probe{ProbeHandler: corev1.ProbeHandler{HTTPGet: &corev1.HTTPGetAction{Path: path, Port: intstr.FromString("http")}}, InitialDelaySeconds: initialDelay, PeriodSeconds: period, TimeoutSeconds: 3}
}

func configEnv(name, configMapName, key string) corev1.EnvVar {
	return corev1.EnvVar{Name: name, ValueFrom: &corev1.EnvVarSource{ConfigMapKeyRef: &corev1.ConfigMapKeySelector{LocalObjectReference: corev1.LocalObjectReference{Name: configMapName}, Key: key}}}
}

func secretEnv(name, secretName, key string) corev1.EnvVar {
	return corev1.EnvVar{Name: name, ValueFrom: &corev1.EnvVarSource{SecretKeyRef: &corev1.SecretKeySelector{LocalObjectReference: corev1.LocalObjectReference{Name: secretName}, Key: key}}}
}

func setCondition(conditions *[]metav1.Condition, conditionType string, ok bool, observedGeneration int64, reason, message string) {
	status := metav1.ConditionFalse
	if ok {
		status = metav1.ConditionTrue
	}
	apimeta.SetStatusCondition(conditions, metav1.Condition{Type: conditionType, Status: status, ObservedGeneration: observedGeneration, Reason: reason, Message: message})
}

func conditionMessage(ok bool, readyMessage, notReadyMessage string) string {
	if ok {
		return readyMessage
	}
	return notReadyMessage
}

func int32Ptr(v int32) *int32                                    { return &v }
func int64Ptr(v int64) *int64                                    { return &v }
func boolPtr(v bool) *bool                                       { return &v }
func pathTypePtr(v networkingv1.PathType) *networkingv1.PathType { return &v }
