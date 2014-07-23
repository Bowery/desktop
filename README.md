# Bowery Desktop Client

Built for OS X only at the moment.

## Development

The desktop client is comprised of an app server and a desktop application.
To develop the application, use XCode. The app server is written in Go,
and works as expected.

Once you boot up the application, it will point to localhost:32055; the changes
you make on the app server will reflect there.

## Compiling Server

In order for the OS X app to launch the server, the binary must be in the right
place. In order to do that, make sure you're in `bowery/client` and run make.
