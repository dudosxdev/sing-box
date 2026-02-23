package mkcp

import (
	"fmt"

	"github.com/sagernet/sing-box/option"
)

// buildMasks creates a list of Udpmask from options.
func buildMasks(masks []option.V2RayMKCPMaskOptions) ([]Udpmask, error) {
	result := make([]Udpmask, 0, len(masks))
	for _, m := range masks {
		mask, err := buildMask(m)
		if err != nil {
			return nil, err
		}
		result = append(result, mask)
	}
	return result, nil
}

func buildMask(m option.V2RayMKCPMaskOptions) (Udpmask, error) {
	switch m.Type {
	case "original", "":
		return newOriginalMask(), nil
	case "aes128gcm":
		return newAes128GcmMask(m.Password), nil
	case "srtp":
		return newSRTPMask(), nil
	case "utp":
		return newUTPMask(), nil
	case "wechat-video":
		return newWechatMask(), nil
	case "dtls":
		return newDTLSMask(), nil
	case "wireguard":
		return newWireGuardMask(), nil
	case "dns":
		return newDNSMask(m.Domain), nil
	case "salamander":
		return newSalamanderMask(m.Password), nil
	default:
		return nil, fmt.Errorf("unknown mkcp mask type: %s", m.Type)
	}
}

// BuildMasksForTest is exported for testing purposes.
func BuildMasksForTest(masks []option.V2RayMKCPMaskOptions) ([]Udpmask, error) {
	return buildMasks(masks)
}
