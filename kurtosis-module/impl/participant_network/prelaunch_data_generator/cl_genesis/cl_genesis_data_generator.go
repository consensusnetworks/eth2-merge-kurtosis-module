package cl_genesis

import (
	"context"
	"fmt"
	"github.com/kurtosis-tech/eth2-merge-kurtosis-module/kurtosis-module/impl/participant_network/prelaunch_data_generator/el_genesis"
	"github.com/kurtosis-tech/eth2-merge-kurtosis-module/kurtosis-module/impl/participant_network/prelaunch_data_generator/prelaunch_data_generator_launcher"
	"github.com/kurtosis-tech/kurtosis-sdk/api/golang/core/lib/enclaves"
	"github.com/kurtosis-tech/kurtosis-sdk/api/golang/core/lib/services"
	"github.com/kurtosis-tech/stacktrace"
	"github.com/sirupsen/logrus"
	"path"
	"strings"
)

const (
	// Needed to copy the JWT secret and the EL genesis.json file
	elGenesisDirpathOnGenerator = "/el-genesis"

	configDirpathOnGenerator = "/config"
	genesisConfigYmlFilename = "config.yaml" // WARNING: Do not change this! It will get copied to the CL genesis data, and the CL clients are hardcoded to look for this filename
	mnemonicsYmlFilename     = "mnemonics.yaml"
	outputDirpathOnGenerator = "/output"
	tranchesDiranme          = "tranches"
	genesisStateFilename     = "genesis.ssz"
	deployBlockFilename      = "deploy_block.txt"
	depositContractFilename  = "deposit_contract.txt"

	// Generation constants
	clGenesisGenerationBinaryFilepathOnContainer = "/usr/local/bin/eth2-testnet-genesis"
	deployBlock                                  = "0"
	eth1Block                                    = "0x0000000000000000000000000000000000000000000000000000000000000000"

	successfulExecCmdExitCode = 0
)

type clGenesisConfigTemplateData struct {
	NetworkId                          string
	SecondsPerSlot                     uint32
	UnixTimestamp                      uint64
	TotalTerminalDifficulty            uint64
	AltairForkEpoch                    uint64
	MergeForkEpoch                     uint64
	NumValidatorKeysToPreregister      uint32
	PreregisteredValidatorKeysMnemonic string
	DepositContractAddress             string
}

func GenerateCLGenesisData(
	ctx context.Context,
	enclaveCtx *enclaves.EnclaveContext,
	genesisGenerationConfigYmlTemplate string,
	genesisGenerationMnemonicsYmlTemplate string,
	elGenesisData *el_genesis.ELGenesisData, // Needed to get JWT secret
	genesisUnixTimestamp uint64,
	networkId string,
	depositContractAddress string,
	secondsPerSlot uint32,
	preregisteredValidatorKeysMnemonic string,
	totalNumValidatorKeysToPreregister uint32,
) (
	*CLGenesisData,
	error,
) {
	templateData := &clGenesisConfigTemplateData{
		NetworkId:                          networkId,
		SecondsPerSlot:                     secondsPerSlot,
		UnixTimestamp:                      genesisUnixTimestamp,
		NumValidatorKeysToPreregister:      totalNumValidatorKeysToPreregister,
		PreregisteredValidatorKeysMnemonic: preregisteredValidatorKeysMnemonic,
		DepositContractAddress:             depositContractAddress,
	}

	genesisGenerationMnemonicsTemplateAndData := enclaves.NewTemplateAndData(genesisGenerationMnemonicsYmlTemplate, templateData)
	genesisGenerationConfigTemplateAndData := enclaves.NewTemplateAndData(genesisGenerationConfigYmlTemplate, templateData)

	templateAndDataByRelDestFilepath := make(map[string]*enclaves.TemplateAndData)
	templateAndDataByRelDestFilepath[mnemonicsYmlFilename] = genesisGenerationMnemonicsTemplateAndData
	templateAndDataByRelDestFilepath[genesisConfigYmlFilename] = genesisGenerationConfigTemplateAndData

	genesisGenerationConfigArtifactUuid, err := enclaveCtx.RenderTemplates(templateAndDataByRelDestFilepath)
	if err != nil {
		return nil, stacktrace.Propagate(err, "An error occurred rendering the CL genesis generation template files")
	}

	// TODO Make this the actual data generator
	serviceCtx, err := prelaunch_data_generator_launcher.LaunchPrelaunchDataGenerator(
		enclaveCtx,
		map[services.FilesArtifactUUID]string{
			genesisGenerationConfigArtifactUuid:  configDirpathOnGenerator,
			elGenesisData.GetFilesArtifactUUID(): elGenesisDirpathOnGenerator,
		},
	)
	if err != nil {
		return nil, stacktrace.Propagate(err, "An error occurred launching the generator container")
	}
	defer func() {
		serviceId := serviceCtx.GetServiceID()
		if err := enclaveCtx.RemoveService(serviceId, 0); err != nil {
			logrus.Warnf("Tried to remove prelaunch data generator service '%v', but doing so threw an error:\n%v", serviceId, err)
		}
	}()

	allDirpathsToCreateOnGenerator := []string{
		configDirpathOnGenerator,
		outputDirpathOnGenerator,
	}
	allDirpathCreationCommands := []string{}
	for _, dirpathToCreateOnGenerator := range allDirpathsToCreateOnGenerator {
		allDirpathCreationCommands = append(
			allDirpathCreationCommands,
			fmt.Sprintf("mkdir -p %v", dirpathToCreateOnGenerator),
		)
	}
	dirCreationCmd := []string{
		"bash",
		"-c",
		strings.Join(allDirpathCreationCommands, " && "),
	}
	if err := execCommand(serviceCtx, dirCreationCmd); err != nil {
		return nil, stacktrace.Propagate(
			err,
			"An error occurred executing dir creation command '%+v' on the generator container",
			dirCreationCmd,
		)
	}

	// Copy files to output
	allFilepathsToCopyToOuptutDirectory := []string{
		path.Join(configDirpathOnGenerator, genesisConfigYmlFilename),
		path.Join(configDirpathOnGenerator, mnemonicsYmlFilename),
		path.Join(elGenesisDirpathOnGenerator, elGenesisData.GetJWTSecretRelativeFilepath()),
	}

	for _, filepathOnGenerator := range allFilepathsToCopyToOuptutDirectory {
		cmd := []string{
			"cp",
			filepathOnGenerator,
			outputDirpathOnGenerator,
		}
		if err := execCommand(serviceCtx, cmd); err != nil {
			return nil, stacktrace.Propagate(
				err,
				"An error occurred executing command '%+v' to copy a file to CL genesis output directory '%v'",
				cmd,
				outputDirpathOnGenerator,
			)
		}
	}

	// Generate files that need dynamic content
	contentToWriteToOutputFilename := map[string]string{
		deployBlock:            deployBlockFilename,
		depositContractAddress: depositContractFilename,
	}
	for content, destFilename := range contentToWriteToOutputFilename {
		destFilepath := path.Join(outputDirpathOnGenerator, destFilename)
		cmd := []string{
			"sh",
			"-c",
			fmt.Sprintf(
				"echo %v > %v",
				content,
				destFilepath,
			),
		}
		if err := execCommand(serviceCtx, cmd); err != nil {
			return nil, stacktrace.Propagate(
				err,
				"An error occurred executing command '%+v' to write content '%v' to file '%v'",
				cmd,
				content,
				destFilepath,
			)
		}
	}

	clGenesisGenerationCmdArgs := []string{
		clGenesisGenerationBinaryFilepathOnContainer,
		"merge",
		"--config", path.Join(outputDirpathOnGenerator, genesisConfigYmlFilename),
		"--mnemonics", path.Join(outputDirpathOnGenerator, mnemonicsYmlFilename),
		"--eth1-config", path.Join(elGenesisDirpathOnGenerator, elGenesisData.GetGethGenesisJsonRelativeFilepath()),
		"--tranches-dir", path.Join(outputDirpathOnGenerator, tranchesDiranme),
		"--state-output", path.Join(outputDirpathOnGenerator, genesisStateFilename),
	}
	if err := execCommand(serviceCtx, clGenesisGenerationCmdArgs); err != nil {
		return nil, stacktrace.Propagate(
			err,
			"An error occurred executing command '%+v' to generate CL genesis data in directory '%v'",
			clGenesisGenerationCmdArgs,
			outputDirpathOnGenerator,
		)
	}

	clGenesisDataArtifactUuid, err := enclaveCtx.StoreServiceFiles(ctx, serviceCtx.GetServiceID(), outputDirpathOnGenerator)
	if err != nil {
		return nil, stacktrace.Propagate(
			err,
			"An error occurred storing the CL genesis files at '%v' in service '%v'",
			outputDirpathOnGenerator,
			serviceCtx.GetServiceID(),
		)
	}

	jwtSecretRelFilepath := path.Join(
		path.Base(outputDirpathOnGenerator),
		path.Base(elGenesisData.GetJWTSecretRelativeFilepath()),
	)
	genesisConfigRelFilepath := path.Join(
		path.Base(outputDirpathOnGenerator),
		genesisConfigYmlFilename,
	)
	genesisSszRelFilepath := path.Join(
		path.Base(outputDirpathOnGenerator),
		genesisStateFilename,
	)
	result := newCLGenesisData(
		clGenesisDataArtifactUuid,
		jwtSecretRelFilepath,
		genesisConfigRelFilepath,
		genesisSszRelFilepath,
	)

	return result, nil
}

func execCommand(serviceCtx *services.ServiceContext, cmd []string) error {
	exitCode, output, err := serviceCtx.ExecCommand(cmd)
	if err != nil {
		return stacktrace.Propagate(
			err,
			"An error occurred executing command '%+v' on the generator container",
			cmd,
		)
	}
	if exitCode != successfulExecCmdExitCode {
		return stacktrace.NewError(
			"Command '%+v' should have returned %v but returned %v with the following output:\n%v",
			cmd,
			successfulExecCmdExitCode,
			exitCode,
			output,
		)
	}
	return nil
}
