// Copyright 2014 Bowery, Inc.
// Lists all plugin events and details under what circumstances
// they are triggered.
package plugin

const (
	// "Initialization" occurs after the plugin has been installed and when the agent
	// reboots and reloads all previously installed plugins.
	ON_PLUGIN_INIT = "on-plugin-init"

	// Triggered after the plugin has been initialized. It is a background
	// process that runs for the lifetime of the agent. It is restarted
	// whenever the plugin is initialized (e.g. agent reboot). It has
	// access to the stdout/stderr of the application via files
	// found at $STDOUT and $STDERR.
	BACKGROUND = "background"

	// Triggered before restarting the application. At this point
	// the application's files have been synced.
	BEFORE_APP_RESTART = "before-app-restart"

	// Triggered after the build step and start step have been
	// executed.
	AFTER_APP_RESTART = "after-app-restart"

	// Triggered before handling an application update.
	BEFORE_APP_UPDATE = "before-app-update"

	// Triggered after handling an application update.
	AFTER_APP_UPDATE = "after-app-update"

	// Triggered before an application is removed.
	BEFORE_APP_DELETE = "before-app-delete"

	// Triggered after an application is removed.
	// Expected use: cleanup.
	AFTER_APP_DELETE = "after-app-delete"

	// Triggered before handling a file update.
	BEFORE_FILE_UPDATE = "before-file-update"

	// Triggered after handling a file update. At this point
	// the application's files have been synced.
	AFTER_FILE_UPDATE = "after-file-update"

	// Triggered before handling a file create.
	BEFORE_FILE_CREATE = "before-file-create"

	// Triggered after handling a file create. At this point
	// the application's files have been synced.
	AFTER_FILE_CREATE = "after-file-create"

	// Triggered after handling a file delete.
	BEFORE_FILE_DELETE = "before-file-delete"

	// Triggered after handling a file delete. At this point
	// the application's files have been synced.
	AFTER_FILE_DELETE = "after-file-delete"

	// Triggered before handling a full upload. Full uploads
	// occur when a new application is created and when
	// the client is started.
	BEFORE_FULL_UPLOAD = "before-full-upload"

	// Triggered after handling a full upload. Full uploads
	// occur when a new application is created and when
	// the client is started.
	AFTER_FULL_UPLOAD = "after-full-upload"
)
