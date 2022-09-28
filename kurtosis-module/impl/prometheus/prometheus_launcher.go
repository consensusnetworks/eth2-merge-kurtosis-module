package prometheus

import (
	"fmt"
	"github.com/kurtosis-tech/eth2-merge-kurtosis-module/kurtosis-module/impl/participant_network/cl"
	"github.com/kurtosis-tech/kurtosis-sdk/api/golang/core/lib/enclaves"
	"github.com/kurtosis-tech/kurtosis-sdk/api/golang/core/lib/services"
	"github.com/kurtosis-tech/stacktrace"
	"path"
)

const (
	serviceID = "prometheus"
	imageName = "prom/prometheus:latest" //TODO I'm not sure if we should use latest version or ping an specific version instead

	httpPortId            = "http"
	httpPortNumber uint16 = 9090

	configFilename = "prometheus-config.yml"

	configDirMountpointOnPrometheus = "/config"
)

var usedPorts = map[string]*services.PortSpec{
	httpPortId: services.NewPortSpec(httpPortNumber, services.PortProtocol_TCP),
}

type configTemplateData struct {
	CLNodesMetricsInfo []*cl.CLNodeMetricsInfo
}

func LaunchPrometheus(
	enclaveCtx *enclaves.EnclaveContext,
	configTemplate string,
	clClientContexts []*cl.CLClientContext,
) (string, error) {
	allCLNodesMetricsInfo := []*cl.CLNodeMetricsInfo{}
	for _, clClientCtx := range clClientContexts {
		if clClientCtx.GetNodesMetricsInfo() != nil {
			allCLNodesMetricsInfo = append(allCLNodesMetricsInfo, clClientCtx.GetNodesMetricsInfo()...)
		}
	}

	templateData := configTemplateData{
		CLNodesMetricsInfo: allCLNodesMetricsInfo,
	}

	templateAndData := enclaves.NewTemplateAndData(configTemplate, templateData)
	templateAndDataByDestRelFilepath := make(map[string]*enclaves.TemplateAndData)
	templateAndDataByDestRelFilepath[configFilename] = templateAndData

	configArtifactUuid, err := enclaveCtx.RenderTemplates(templateAndDataByDestRelFilepath)
	if err != nil {
		return "", stacktrace.Propagate(err, "An error occurred rendering the Prometheus template config file to '%v'", configFilename)
	}

	containerConfig := getContainerConfig(configArtifactUuid)
	serviceCtx, err := enclaveCtx.AddService(serviceID, containerConfig)
	if err != nil {
		return "", stacktrace.Propagate(err, "An error occurred launching the prometheus service")
	}

	privateIpAddr := serviceCtx.GetPrivateIPAddress()
	privateHttpPort, found := serviceCtx.GetPrivatePorts()[httpPortId]
	if !found {
		return "", stacktrace.NewError("Expected the newly-started prometheus service to have a private port with ID '%v' but none was found", httpPortId)
	}

	privateUrl := fmt.Sprintf("http://%v:%v", privateIpAddr, privateHttpPort.GetNumber())
	return privateUrl, nil
}

// ====================================================================================================
//                                       Private Helper Functions
//
// ====================================================================================================
func getContainerConfig(
	configFileArtifactUuid services.FilesArtifactUUID,
) *services.ContainerConfig {
	configFilepath := path.Join(configDirMountpointOnPrometheus, path.Base(configFilename))
	containerConfig := services.NewContainerConfigBuilder(
		imageName,
	).WithCmdOverride([]string{
		//You can check all the cli flags starting the container and going to the flags section
		//in Prometheus admin page "{{prometheusPublicURL}}/flags" section
		"--config.file=" + configFilepath,
		"--storage.tsdb.path=/prometheus",
		"--storage.tsdb.retention.time=1d",
		"--storage.tsdb.retention.size=512MB",
		"--storage.tsdb.wal-compression",
		"--web.console.libraries=/etc/prometheus/console_libraries",
		"--web.console.templates=/etc/prometheus/consoles",
		"--web.enable-lifecycle",
	}).WithUsedPorts(
		usedPorts,
	).WithFiles(map[services.FilesArtifactUUID]string{
		configFileArtifactUuid: configDirMountpointOnPrometheus,
	}).Build()

	return containerConfig
}
