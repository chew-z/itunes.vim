function run(argv) {
    const Music = Application('Music')

    if (argv.length !== 1) {
        return JSON.stringify({ error: 'A single device name argument is required.' })
    }

    const targetDeviceName = argv[0]

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
                // Use default
            }
        }

        // If targeting local/computer, disable AirPlay
        if (targetDeviceName === computerName || targetDeviceName.toLowerCase() === 'local' || targetDeviceName.toLowerCase() === 'computer') {
            try {
                // Disable AirPlay to switch to local output
                Music.airPlayEnabled = false

                return JSON.stringify({
                    name: computerName,
                    kind: 'computer',
                    selected: true,
                    sound_volume: Music.soundVolume(),
                })
            } catch (e) {
                return JSON.stringify({
                    error: `Failed to set local audio: ${e.toString()}`,
                })
            }
        }

        // For AirPlay devices, we can't directly enumerate or select them due to privilege restrictions
        // We can only inform the user about this limitation
        return JSON.stringify({
            error: 'Setting specific AirPlay devices is not supported due to system restrictions',
            message: 'You can only switch between local output and the currently connected AirPlay device',
            hint: 'To switch to local output, use "local" or "' + computerName + '" as the device name',
        })
    } catch (e) {
        return JSON.stringify({
            error: `An error occurred: ${e.toString()}`,
        })
    }
}
