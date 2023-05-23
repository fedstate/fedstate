package core

type mongoCommand int

var staticMongoCommand = new(mongoCommand)

func (*mongoCommand) CommandReplSet(replSet, config string) []string {
	var command = []string{
		"mongod",
		"--port",
		DefaultPortStr,
		"--bind_ip",
		"0.0.0.0",
		"--replSet",
		replSet,
		"--auth",
		"--keyFile",
		keyfilePath,
	}
	if config != "" {
		return append(command, "--config",
			mongodConfigPath)
	}
	return command
}
