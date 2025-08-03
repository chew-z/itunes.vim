function run() {
    const Music = Application('Music')

    if (!Music.running()) {
        return JSON.stringify({ error: 'Music app is not running.' })
    }

    try {
        // Get the computer name for local output
        const app = Application.currentApplication()
        app.includeStandardAdditions = true
        let computerName = 'Computer'

        try {
            computerName = app.doShellScript('scutil --get ComputerName')
        } catch (e) {
            try {
                computerName = app.doShellScript('hostname -s')
            } catch (e2) {
                // Fallback to default
            }
        }

        // Try to get AirPlay devices
        try {
            // Check if AirPlay is enabled first
            const airPlayEnabled = Music.airPlayEnabled()

            if (airPlayEnabled) {
                // Get current AirPlay devices
                const currentDevices = Music.currentAirPlayDevices()

                if (currentDevices && currentDevices.length > 0) {
                    // Get the first (and usually only) selected AirPlay device
                    const selectedDevice = currentDevices[0]
                    return JSON.stringify({
                        output_type: 'airplay',
                        device_name: selectedDevice.name(),
                    })
                }
            }
        } catch (e) {
            // AirPlay might not be available or accessible
            // Fall through to local output
        }

        // If we get here, output is local
        return JSON.stringify({
            output_type: 'local',
            device_name: computerName,
        })
    } catch (e) {
        // On any error, assume local output
        return JSON.stringify({
            output_type: 'local',
            device_name: 'Computer',
            warning: 'Could not determine audio output, defaulting to local',
        })
    }
}
