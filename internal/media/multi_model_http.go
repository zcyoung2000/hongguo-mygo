package media

import (
	"net/http"
	"strconv"
)

const multiModelReadingRequest = "739321001"

func defaultPlaybackDeviceQuery(deviceID, installID, cdid, openudid string) [][2]string {
	return [][2]string{
		{"iid", installID},
		{"device_id", deviceID},
		{"ac", "wifi"},
		{"channel", "xiaomi_8662_64"},
		{"aid", "8662"},
		{"app_name", "novelread"},
		{"cdid", cdid},
		{"version_code", "73932"},
		{"version_name", sdkVersionName},
		{"device_platform", "android"},
		{"os", "android"},
		{"ssmix", "a"},
		{"device_type", "23127PN0CC"},
		{"device_brand", "Xiaomi"},
		{"language", "zh"},
		{"os_api", "34"},
		{"os_version", "14"},
		{"manifest_version_code", "73932"},
		{"resolution", "1200*2670"},
		{"dpi", "460"},
		{"update_version_code", "73932"},
		{"host_abi", "arm64-v8a"},
		{"dragon_device_type", "phone"},
		{"pv_player", "73932"},
		{"compliance_status", "0"},
		{"need_personal_recommend", "1"},
		{"player_so_load", "1"},
		{"is_android_pad_screen", "0"},
		{"openudid", openudid},
	}
}

func setPlaybackHeaders(header http.Header, ticket int64) {
	header.Set("x-ss-dp", "8662")
	header.Set("lc", "101")
	header.Set("x-reading-request", multiModelReadingRequest)
	header.Set("x-ss-req-ticket", strconv.FormatInt(ticket, 10))
}
