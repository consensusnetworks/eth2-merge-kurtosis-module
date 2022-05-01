package prelaunch_data_generator

import (
	"fmt"
	"github.com/kurtosis-tech/eth2-merge-kurtosis-module/kurtosis-module/impl/prelaunch_data_generator/cl_genesis"
	"github.com/kurtosis-tech/eth2-merge-kurtosis-module/kurtosis-module/impl/prelaunch_data_generator/cl_validator_keystores"
	"github.com/kurtosis-tech/kurtosis-core-api-lib/api/golang/lib/enclaves"
	"github.com/kurtosis-tech/kurtosis-core-api-lib/api/golang/lib/services"
	"github.com/kurtosis-tech/stacktrace"
	"time"
)

const (
	// Though this is a Kurtosis image, it's actually built from the original repo:
	//  https://github.com/skylenet/ethereum-genesis-generator
	// It's only a Kurtosis image because the original repo doesn't publish Docker images
	image = "skylenet/ethereum-genesis-generator:latest"

	serviceIdPrefix = "prelaunch-data-generator-"
)

// We use Docker exec commands to run the commands we need, so we override the default
var entrypointArgs = []string{
	"sleep",
	"999999",
}

type PrelaunchData struct {
	GethELGenesisJsonFilepathOnModuleContainer     string
	NethermindGenesisJsonFilepathOnModuleContainer string
	CLGenesisPaths                                 *cl_genesis.CLGenesisData
	KeystoresGenerationResult                      *cl_validator_keystores.GenerateKeystoresResult
}

func LaunchPrelaunchDataGenerator(
	enclaveCtx *enclaves.EnclaveContext,
	filesArtifactMountpoints map[services.FilesArtifactID]string,

	/*
	networkId string,
	depositContractAddress string,
	totalTerminalDifficulty uint64,
	preregisteredValidatorKeysMnemonic string,

	 */
) (
	*services.ServiceContext,
	error,
) {
	containerConfigSupplier := getContainerConfigSupplier(filesArtifactMountpoints)

	serviceId := services.ServiceID(fmt.Sprintf(
		"%v%v",
		serviceIdPrefix,
		time.Now().Unix(),
	))

	serviceCtx, err := enclaveCtx.AddService(serviceId, containerConfigSupplier)
	if err != nil {
		return nil, stacktrace.Propagate(err, "An error occurred launching the prelaunch data generator container with service ID '%v'", serviceIdPrefix)
	}

	return serviceCtx, nil

	/*
	result := newPrelaunchDataGeneratorContext(
		serviceCtx,
		networkId,
		depositContractAddress,
		totalTerminalDifficulty,
		preregisteredValidatorKeysMnemonic,
	)
	return result, nil

	 */
}

func getContainerConfigSupplier(
	filesArtifactMountpoints map[services.FilesArtifactID]string,
) func(privateIpAddr string) (*services.ContainerConfig, error) {
	return func(privateIpAddr string) (*services.ContainerConfig, error) {
		containerConfig := services.NewContainerConfigBuilder(
			image,
		).WithEntrypointOverride(
			entrypointArgs,
		).WithFiles(
			filesArtifactMountpoints,
		).Build()

		return containerConfig, nil
	}
}
