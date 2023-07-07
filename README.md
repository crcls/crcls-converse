## CRCLS Converse

This application is the base layer of CRCLS. It is the gateway to the network.

This is a **hardstop** WIP. So many things to build. ❤️

## Inatallation

To run this app locally on a Mac, make sure you have the latest verison of Xcode commandline tools installed.

`xcode-select -p` To see if they are installed. If not run `xcode-select --install`

Once that is complete, make sure you have Go installed and up to date. The simplest way to manage the installation is with brew..

`brew search golang`

Install the latest verison.

`brew install golang`

Once that's done, verify the the installation with `go version`.

## Running the app

Basic usage is to run the app locally from the repo root using this command:

```bash
go run .
```

Add the `-verbose` flag to see the debug output.

This will start the application and use the console's stdin and stdout for interaction.

When peers join the network, you'll see a JSON event printed to the console.

## Interaction

There are only a couple commands so far. All commands start with a / (slash) and have one or more subcommands.

### Commands:

**/list channels**

This command will respond with a JSON object that lists the registered channels. (So far only the global channel is availble.)

**/join {channelName}**

Join a channel by passing the channel name to this command. Eg. `/join global`

**/reply {message}**

Once you've joined a channel, you can now start sending messages. Use this command with a following message to broadcast. Other connected nodes that are subscribed to this channel will receive your message.

**/list messages [int]**

This command returns the history for the last `[int]` days of the current channel.

**/list members**

Returns a list of the peers connected to the network.

### Beta Commands:

These are more WIP commands.

**/member create {member JSON object}**

Create a new member account. This creates an EVM compatible wallet keypair and saves it to the local filesystem. Plans to include other chains are in the works.
