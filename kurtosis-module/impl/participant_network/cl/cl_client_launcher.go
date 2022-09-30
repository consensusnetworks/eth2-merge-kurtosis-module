package cl

import (
	"github.com/kurtosis-tech/eth2-merge-kurtosis-module/kurtosis-module/impl/module_io"
	"github.com/kurtosis-tech/eth2-merge-kurtosis-module/kurtosis-module/impl/participant_network/el"
	"github.com/kurtosis-tech/eth2-merge-kurtosis-module/kurtosis-module/impl/participant_network/mev_boost"
	"github.com/kurtosis-tech/eth2-merge-kurtosis-module/kurtosis-module/impl/participant_network/prelaunch_data_generator/cl_validator_keystores"
	"github.com/kurtosis-tech/kurtosis-sdk/api/golang/core/lib/enclaves"
	"github.com/kurtosis-tech/kurtosis-sdk/api/golang/core/lib/services"
)

type CLClientLauncher interface {
	// Launches both a Beacon client AND a validator
	Launch(
		enclaveCtx *enclaves.EnclaveContext,
		serviceId services.ServiceID,
		image string,
		participantLogLevel string,
		globalLogLevel module_io.GlobalClientLogLevel,
		// If nil, the node will be launched as a bootnode
		bootnodeContext *CLClientContext,
		elClientContext *el.ELClientContext,
		mevBoostContext *mev_boost.MEVBoostContext,
		nodeKeystoreFiles *cl_validator_keystores.KeystoreFiles,
		extraBeaconParams []string,
		extraValidatorParams []string,
	) (
		resultClientCtx *CLClientContext,
		resultErr error,
	)
}
