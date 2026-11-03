package upstream

func DefaultDeviceQueryPairs(deviceID, installID, cdid, openudid string) [][2]string {
	return [][2]string{
		{"aid", "8662"},
		{"app_name", "novelread"},
		{"version_code", "73932"},
		{"version_name", "7.3.9.32"},
		{"device_platform", "android"},
		{"os_version", "14"},
		{"device_type", "23127PN0CC"},
		{"device_brand", "Xiaomi"},
		{"channel", "xiaomi_8662_64"},
		{"device_id", deviceID},
		{"iid", installID},
		{"cdid", cdid},
		{"openudid", openudid},
		{"dpi", "460"},
		{"update_version_code", "73932"},
	}
}
