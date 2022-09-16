package grafana

import (
	"github.com/kurtosis-tech/eth2-merge-kurtosis-module/kurtosis-module/impl/static_files"
	"github.com/kurtosis-tech/kurtosis-core-api-lib/api/golang/lib/enclaves"
	"github.com/kurtosis-tech/kurtosis-core-api-lib/api/golang/lib/services"
	"github.com/kurtosis-tech/stacktrace"
)

const (
	serviceID = "grafana"
	imageName = "grafana/grafana-enterprise:latest" //TODO I'm not sure if we should use latest version or ping an specific version instead

	httpPortId            = "http"
	httpPortNumber uint16 = 3000

	datasourceConfigRelFilepath = "datasources/datasource.yml"

	// this is relative to the files artifact root
	dashboardProvidersConfigRelFilepath = "dashboards/dashboard-providers.yml"

	configDirpathEnvVar = "GF_PATHS_PROVISIONING"

	grafanaConfigDirpathOnService      = "/config"
	grafanaDashboardsDirpathOnService  = "/dashboards"
	grafanaDashboardsFilepathOnService = grafanaDashboardsDirpathOnService + "/dashboard.json"
)

var usedPorts = map[string]*services.PortSpec{
	httpPortId: services.NewPortSpec(httpPortNumber, services.PortProtocol_TCP),
}

type datasourceConfigTemplateData struct {
	PrometheusURL string
}

type dashboardProvidersConfigTemplateData struct {
	DashboardsDirpath string
}

func LaunchGrafana(
	enclaveCtx *enclaves.EnclaveContext,
	datasourceConfigTemplate string,
	dashboardProvidersConfigTemplate string,
	prometheusPrivateUrl string,
) error {
	grafanaConfig, grafanaDashboards, err := getGrafanaConfigDirArtifactUuid(
		enclaveCtx,
		datasourceConfigTemplate,
		dashboardProvidersConfigTemplate,
		prometheusPrivateUrl,
	)
	if err != nil {
		return stacktrace.Propagate(err, "An error occurred getting the Grafana config directory files artifact")
	}

	containerConfig := getContainerConfig(grafanaConfig, grafanaDashboards)
	_, err = enclaveCtx.AddService(serviceID, containerConfig)
	if err != nil {
		return stacktrace.Propagate(err, "An error occurred launching the grafana service")
	}

	return nil
}

// ====================================================================================================
//
//	Private Helper Functions
//
// ====================================================================================================
func getGrafanaConfigDirArtifactUuid(
	enclaveCtx *enclaves.EnclaveContext,
	datasourceConfigTemplate string,
	dashboardProvidersConfigTemplate string,
	prometheusPrivateUrl string,
) (resultGrafanaConfig services.FilesArtifactUUID, resultGrafanaDashboards services.FilesArtifactUUID, resultErr error) {

	datasourceData := datasourceConfigTemplateData{
		PrometheusURL: prometheusPrivateUrl,
	}
	datasourceTemplateAndData := enclaves.NewTemplateAndData(datasourceConfigTemplate, datasourceData)

	dashboardProvidersData := dashboardProvidersConfigTemplateData{
		// Grafana needs to know where the dashboards config file will be on disk, which means we need to feed
		//  it the *mounted* location on disk (on the Grafana container) when we generate this on the module container
		DashboardsDirpath: grafanaDashboardsFilepathOnService,
	}
	dashboardProvidersTemplateAndData := enclaves.NewTemplateAndData(dashboardProvidersConfigTemplate, dashboardProvidersData)

	templateAndDataByDestRelFilepath := make(map[string]*enclaves.TemplateAndData)
	templateAndDataByDestRelFilepath[datasourceConfigRelFilepath] = datasourceTemplateAndData
	templateAndDataByDestRelFilepath[dashboardProvidersConfigRelFilepath] = dashboardProvidersTemplateAndData

	grafanaConfig, err := enclaveCtx.RenderTemplates(templateAndDataByDestRelFilepath)
	if err != nil {
		return "", "", stacktrace.Propagate(err, "An error occurred rendering Grafana templates")
	}

	grafanaDashboards, err := enclaveCtx.UploadFiles(static_files.GrafanaDashboardsConfigDirpath)
	if err != nil {
		return "", "", stacktrace.Propagate(err, "An error occurred uploading Grafana dashboard.json")
	}

	return grafanaConfig, grafanaDashboards, nil
}

func getContainerConfig(
	grafanaConfig services.FilesArtifactUUID,
	grafanaDashboards services.FilesArtifactUUID,
) *services.ContainerConfig {
	containerConfig := services.NewContainerConfigBuilder(
		imageName,
	).WithUsedPorts(
		usedPorts,
	).WithEnvironmentVariableOverrides(map[string]string{
		configDirpathEnvVar: grafanaConfigDirpathOnService,
	}).WithFiles(map[services.FilesArtifactUUID]string{
		grafanaConfig:     grafanaConfigDirpathOnService,
		grafanaDashboards: grafanaDashboardsDirpathOnService,
	}).Build()

	return containerConfig
}
