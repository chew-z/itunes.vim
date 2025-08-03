package itunes

import _ "embed"

//go:embed scripts/iTunes_Play_Playlist_Track.js
var playScript string

//go:embed scripts/iTunes_Refresh_Library.js
var refreshScript string

//go:embed scripts/iTunes_Now_Playing.js
var nowPlayingScript string

//go:embed scripts/iTunes_Play_Stream_URL.js
var playStreamScript string

//go:embed scripts/iTunes_Get_EQ.js
var getEQScript string

//go:embed scripts/iTunes_Set_EQ.js
var setEQScript string

//go:embed scripts/iTunes_Get_Audio_Output.js
var getAudioOutputScript string

//go:embed scripts/iTunes_List_AirPlay_Devices.js
var listAirPlayDevicesScript string

//go:embed scripts/iTunes_Set_AirPlay_Device.js
var setAirPlayDeviceScript string
