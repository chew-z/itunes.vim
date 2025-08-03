
function run() {
    const Music = Application("com.apple.Music");
    const system = Application("System Events");

    if (!Music.running()) {
        return JSON.stringify({ error: "Music app is not running." });
    }

    try {
        const computerName = system.hostName();
        const selectedDevices = Music.airPlayDevices.whose({ selected: true });

        if (selectedDevices.length === 0) {
            return JSON.stringify({ output_type: "unknown", device_name: null, error: "No audio output device is currently selected." });
        }

        const selectedDevice = selectedDevices[0];
        const deviceName = selectedDevice.name();

        let outputType = "local";
        if (deviceName !== computerName) {
            outputType = "airplay";
        }

        return JSON.stringify({
            output_type: outputType,
            device_name: deviceName
        });

    } catch (e) {
        return JSON.stringify({ error: `An error occurred: ${e.message}` });
    }
}
