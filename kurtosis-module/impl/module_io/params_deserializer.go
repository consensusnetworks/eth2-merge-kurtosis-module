package module_io

import (
	"strings"

	"github.com/kurtosis-tech/stacktrace"
	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v3"
)

const (
	expectedSecondsPerSlot = 12
	expectedSlotsPerEpoch  = 32
)

func DeserializeAndValidateParams(paramsStr string) (*ExecuteParams, error) {
	paramsObj := GetDefaultExecuteParams()
	if err := yaml.Unmarshal([]byte(paramsStr), paramsObj); err != nil {
		return nil, stacktrace.Propagate(err, "An error occurred deserializing the serialized params")
	}

	if _, found := validGlobalClientLogLevels[paramsObj.ClientLogLevel]; !found {
		return nil, stacktrace.NewError("Unrecognized client log level '%v'", paramsObj.ClientLogLevel)
	}

	// Validate participants
	if len(paramsObj.Participants) == 0 {
		return nil, stacktrace.NewError("At least one participant is required")
	}
	for idx, participant := range paramsObj.Participants {
		if idx == 0 && (participant.ELClientType == ParticipantELClientType_Nethermind || participant.ELClientType == ParticipantELClientType_Besu) {
			return nil, stacktrace.NewError("Cannot use a Nethermind/Besu client for the first participant because Nethermind/Besu clients don't mine on Eth1")
		}

		elClientType := participant.ELClientType
		if _, found := validParticipantELClientTypes[elClientType]; !found {
			validClientTypes := []string{}
			for clientType := range validParticipantELClientTypes {
				validClientTypes = append(validClientTypes, string(clientType))
			}
			return nil, stacktrace.NewError(
				"Participant %v declares unrecognized EL client type '%v'; valid values are: %v",
				idx,
				elClientType,
				strings.Join(validClientTypes, ", "),
			)
		}
		if participant.ELClientImage == useDefaultElImageKeyword {
			defaultElClientImage, found := defaultElImages[elClientType]
			if !found {
				return nil, stacktrace.NewError("EL client image wasn't provided, and no default image was defined for EL client type '%v'; this is a bug in the module", elClientType)
			}
			// Go's "range" is by-value, so we need to actually by-reference modify the paramsObj we need to
			//  use the idx
			paramsObj.Participants[idx].ELClientImage = defaultElClientImage
		}
		if participant.ELExtraParams == nil {
			paramsObj.Participants[idx].ELExtraParams = []string{}
		}

		clClientType := participant.CLClientType
		if _, found := validParticipantCLClientTypes[clClientType]; !found {
			validClientTypes := []string{}
			for clientType := range validParticipantCLClientTypes {
				validClientTypes = append(validClientTypes, string(clientType))
			}
			return nil, stacktrace.NewError(
				"Participant %v declares unrecognized CL client type '%v'; valid values are: %v",
				idx,
				clClientType,
				strings.Join(validClientTypes, ", "),
			)
		}
		if participant.CLClientImage == useDefaultClImageKeyword {
			defaultClClientImage, found := defaultClImages[clClientType]
			if !found {
				return nil, stacktrace.NewError("CL client image wasn't provided, and no default image was defined for CL client type '%v'; this is a bug in the module", clClientType)
			}
			// Go's "range" is by-value, so we need to actually by-reference modify the paramsObj we need to
			//  use the idx
			paramsObj.Participants[idx].CLClientImage = defaultClClientImage
		}
		if participant.BeaconExtraParams == nil {
			paramsObj.Participants[idx].BeaconExtraParams = []string{}
		}
		if participant.ValidatorExtraParams == nil {
			paramsObj.Participants[idx].ValidatorExtraParams = []string{}
		}
	}

	networkParams := paramsObj.Network
	if len(strings.TrimSpace(networkParams.NetworkID)) == 0 {
		return nil, stacktrace.NewError("Network ID must not be empty")
	}
	if len(strings.TrimSpace(networkParams.PremineAccountMnemonic)) == 0 {
		return nil, stacktrace.NewError("Account mnemonic must not be empty")
	}
	if len(strings.TrimSpace(networkParams.DepositContractAddress)) == 0 {
		return nil, stacktrace.NewError("Deposit contract address must not be empty")
	}

	// Slot/epoch validation
	if networkParams.SecondsPerSlot == 0 {
		return nil, stacktrace.NewError("Each slot must be >= 1 second")
	}
	if networkParams.SecondsPerSlot != expectedSecondsPerSlot {
		logrus.Warnf("The current seconds-per-slot value is set to '%v'; values that aren't '%v' may cause the network to behave strangely", networkParams.SecondsPerSlot, expectedSecondsPerSlot)
	}
	if networkParams.SlotsPerEpoch == 0 {
		return nil, stacktrace.NewError("Each epoch must be composed of >= 1 slot")
	}
	if networkParams.SlotsPerEpoch != expectedSlotsPerEpoch {
		logrus.Warnf("The current slots-per-epoch value is set to '%v'; values that aren't '%v' may cause the network to behave strangely", networkParams.SlotsPerEpoch, expectedSlotsPerEpoch)
	}

	// Validator validation
	requiredNumValidators := 2 * networkParams.SlotsPerEpoch
	actualNumValidators := uint32(len(paramsObj.Participants)) * networkParams.NumValidatorKeysPerNode
	if actualNumValidators < requiredNumValidators {
		return nil, stacktrace.NewError(
			"We need %v validators (enough for two epochs, with one validator per slot), but only have %v",
			requiredNumValidators,
			actualNumValidators,
		)
	}
	if len(strings.TrimSpace(networkParams.PreregisteredValidatorKeysMnemonic)) == 0 {
		return nil, stacktrace.NewError("Preregistered validator keys mnemonic must not be empty")
	}

	// TODO Remove this once Nethermind no longer breaks if there's only one bootnode
	participantParams := paramsObj.Participants
	hasNethermindAsBootnodeOrSecondNode := (len(participantParams) >= 1 && participantParams[0].ELClientType == ParticipantELClientType_Nethermind) ||
		(len(participantParams) >= 2 && participantParams[1].ELClientType == ParticipantELClientType_Nethermind)
	if hasNethermindAsBootnodeOrSecondNode {
		return nil, stacktrace.NewError(
			"Due to a bug in Nethermind peering, Nethermind cannot be either the first or second EL node (see https://discord.com/channels/783719264308953108/933134266580234290/958049716065665094)",
		)
	}

	return paramsObj, nil
}
