package model

// Constants for a community Keycloak installation
const (
	ApplicationName                      = "keycloak"
	MonitoringKey                        = "middleware"
	DatabaseSecretName                   = ApplicationName + "-db-secret"
	PostgresqlPersistentVolumeName       = ApplicationName + "-postgresql-claim"
	PostgresqlBackupPersistentVolumeName = ApplicationName + "-backup"
	PostgresqlDeploymentName             = ApplicationName + "-postgresql"
	KeycloakProbesName                   = ApplicationName + "-probes"
	KeycloakMetricsRouteName             = ApplicationName + "-metrics-rewrite"
	KeycloakMetricsRoutePath             = "/auth/realms/master/metrics"
	KeycloakMetricsRouteRewritePath      = "/auth/realms/master"
	PostgresqlDeploymentComponent        = "database"
	PostgresqlServiceName                = ApplicationName + "-postgresql"
	KeycloakDiscoveryServiceName         = ApplicationName + "-discovery"
	KeycloakMonitoringServiceName        = ApplicationName + "-monitoring"
	KeycloakDeploymentName               = ApplicationName
	KeycloakDeploymentComponent          = "keycloak"
	PostgresqlBackupComponent            = "database-backup"
	PostgresqlDatabase                   = "root"
	PostgresqlUsername                   = ApplicationName
	PostgresqlPasswordLength             = 32
	PostgresqlPersistentVolumeCapacity   = "1Gi"
	PostgresqlPersistentVolumeMountPath  = "/var/lib/pgsql/data"
	DatabaseSecretUsernameProperty       = "POSTGRES_USERNAME"
	DatabaseSecretPasswordProperty       = "POSTGRES_PASSWORD"
	// Required by the Integreately Backup Image
	DatabaseSecretHostProperty = "POSTGRES_HOST"
	// Required by the Integreately Backup Image
	DatabaseSecretDatabaseProperty = "POSTGRES_DATABASE"
	// Required by the Integreately Backup Image
	DatabaseSecretVersionProperty              = "POSTGRES_VERSION"
	DatabaseSecretExternalAddressProperty      = "POSTGRES_EXTERNAL_ADDRESS"
	DatabaseSecretExternalPortProperty         = "POSTGRES_EXTERNAL_PORT"
	KeycloakServicePort                        = 8443
	PostgresDefaultPort                        = 5432
	AdminUsernameProperty                      = "ADMIN_USERNAME"
	AdminPasswordProperty                      = "ADMIN_PASSWORD"
	ServingCertSecretName                      = "sso-x509-https-secret"
	LivenessProbeProperty                      = "liveness_probe.sh"
	ReadinessProbeProperty                     = "readiness_probe.sh"
	RouteLoadBalancingStrategy                 = "source"
	IngressDefaultHost                         = "keycloak.local"
	PostgresqlBackupServiceAccountName         = "keycloak-operator"
	KeycloakExtensionEnvVar                    = "KEYCLOAK_EXTENSIONS"
	KeycloakExtensionPath                      = "/opt/jboss/keycloak/standalone/deployments"
	KeycloakExtensionsInitContainerPath        = "/opt/extensions"
	RhssoExtensionPath                         = "/opt/eap/standalone/deployments"
	ClientSecretName                           = ApplicationName + "-client-secret"
	ClientSecretClientIDProperty               = "CLIENT_ID"
	ClientSecretClientSecretProperty           = "CLIENT_SECRET"
	MaxUnavailableNumberOfPods                 = 1
	ServiceMonitorName                         = ApplicationName + "-service-monitor"
	MigrateBackupName                          = "migrate-backup"
	DatabaseSecretSslModeProperty              = "SSLMODE"
	DatabaseSecretSslCert                      = ApplicationName + "-db-ssl-cert-secret"
	RhssoDatabaseXAConnectionParamsProperty    = "DB_XA_CONNECTION_PROPERTY"
	RhssoDatabaseNONXAConnectionParamsProperty = "DB_CONNECTION_PROPERTY"
	KeycloakDatabaseConnectionParamsProperty   = "JDBC_PARAMS"
	KeycloakCertificatePath                    = "/opt/jboss/.postgresql"
	RhssoCertificatePath                       = "/home/jboss/.postgresql"
)

var PodLabels = map[string]string{}
