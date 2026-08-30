package db

// schemaStatements contains only the SQLite DDL needed by the desktop chat
// application. Runtime CRUD is kept in the chat storage adapter so the
// protocol and persistence layer remain independent from the removed demo
// scaffold.
var schemaStatements = []string{
	`CREATE TABLE IF NOT EXISTS schema_migrations (
		version INTEGER PRIMARY KEY,
		applied_at TEXT NOT NULL
	)`,
	`CREATE TABLE IF NOT EXISTS profiles (
		id INTEGER PRIMARY KEY CHECK(id = 1),
		nickname TEXT NOT NULL DEFAULT '',
		avatar_path TEXT NOT NULL DEFAULT '',
		avatar_hash TEXT NOT NULL DEFAULT '',
		avatar_version INTEGER NOT NULL DEFAULT 0,
		discoverable INTEGER NOT NULL DEFAULT 0,
		auto_save INTEGER NOT NULL DEFAULT 0,
		file_save_path TEXT NOT NULL DEFAULT '',
		shared_root_path TEXT NOT NULL DEFAULT '',
		shared_enabled INTEGER NOT NULL DEFAULT 0,
		shared_drive_multi_window INTEGER NOT NULL DEFAULT 1,
		theme TEXT NOT NULL DEFAULT 'system',
		launch_at_startup INTEGER NOT NULL DEFAULT 0,
		created_at TEXT NOT NULL,
		updated_at TEXT NOT NULL
	)`,
	`CREATE TABLE IF NOT EXISTS device_identity (
		id INTEGER PRIMARY KEY CHECK(id = 1),
		device_id TEXT NOT NULL UNIQUE,
		public_key_pem TEXT NOT NULL,
		private_key_pem TEXT NOT NULL,
		certificate_pem TEXT NOT NULL,
		certificate_fingerprint TEXT NOT NULL,
		created_at TEXT NOT NULL,
		updated_at TEXT NOT NULL
	)`,
	`CREATE TABLE IF NOT EXISTS peers (
		device_id TEXT PRIMARY KEY,
		nickname TEXT NOT NULL DEFAULT '',
		avatar_path TEXT NOT NULL DEFAULT '',
		avatar_hash TEXT NOT NULL DEFAULT '',
		avatar_version INTEGER NOT NULL DEFAULT 0,
		platform TEXT NOT NULL DEFAULT '',
		os_version TEXT NOT NULL DEFAULT '',
		ip TEXT NOT NULL DEFAULT '',
		port INTEGER NOT NULL DEFAULT 0,
		public_key_pem TEXT NOT NULL DEFAULT '',
		certificate_fingerprint TEXT NOT NULL DEFAULT '',
		relation TEXT NOT NULL DEFAULT 'discovered',
		remark TEXT NOT NULL DEFAULT '',
		protocol_name TEXT NOT NULL DEFAULT '',
		protocol_major INTEGER NOT NULL DEFAULT 0,
		discovery_magic TEXT NOT NULL DEFAULT '',
		capabilities TEXT NOT NULL DEFAULT '',
		discovery_visible INTEGER NOT NULL DEFAULT 0,
		visible_in_friends INTEGER NOT NULL DEFAULT 1,
		relationship_version TEXT NOT NULL DEFAULT '',
		last_seen TEXT NOT NULL DEFAULT '',
		created_at TEXT NOT NULL,
		updated_at TEXT NOT NULL
	)`,
	`CREATE INDEX IF NOT EXISTS idx_peers_relation ON peers(relation)`,
	`CREATE TABLE IF NOT EXISTS hidden_friend_devices (
		device_id TEXT PRIMARY KEY,
		hidden_at TEXT NOT NULL
	)`,
	`CREATE TABLE IF NOT EXISTS friend_requests (
		request_id TEXT PRIMARY KEY,
		device_id TEXT NOT NULL,
		nickname TEXT NOT NULL DEFAULT '',
		message TEXT NOT NULL DEFAULT '',
		status TEXT NOT NULL DEFAULT 'pending',
		direction TEXT NOT NULL DEFAULT '',
		created_at TEXT NOT NULL,
		accepted_at TEXT NOT NULL DEFAULT '',
		updated_at TEXT NOT NULL
	)`,
	`CREATE INDEX IF NOT EXISTS idx_friend_requests_status ON friend_requests(status)`,
	`CREATE TABLE IF NOT EXISTS friend_removals (
		device_id TEXT PRIMARY KEY,
		removed_at TEXT NOT NULL,
		relationship_version TEXT NOT NULL DEFAULT '',
		public_key_pem TEXT NOT NULL DEFAULT '',
		certificate_fingerprint TEXT NOT NULL DEFAULT ''
	)`,
	`CREATE TABLE IF NOT EXISTS conversations (
		conversation_id TEXT PRIMARY KEY,
		peer_device_id TEXT NOT NULL UNIQUE,
		last_message TEXT NOT NULL DEFAULT '',
		last_message_at TEXT NOT NULL DEFAULT '',
		unread_count INTEGER NOT NULL DEFAULT 0,
		pinned INTEGER NOT NULL DEFAULT 0,
		created_at TEXT NOT NULL,
		updated_at TEXT NOT NULL
	)`,
	`CREATE TABLE IF NOT EXISTS messages (
		message_id TEXT PRIMARY KEY,
		conversation_id TEXT NOT NULL,
		sender_device_id TEXT NOT NULL,
		kind TEXT NOT NULL DEFAULT 'text',
		content TEXT NOT NULL DEFAULT '',
		status TEXT NOT NULL DEFAULT 'sent',
		created_at TEXT NOT NULL,
		is_favorite INTEGER NOT NULL DEFAULT 0,
		deleted_at TEXT NOT NULL DEFAULT '',
		quote_message_id TEXT NOT NULL DEFAULT '',
		quote_content TEXT NOT NULL DEFAULT '',
		forwarded_from TEXT NOT NULL DEFAULT '',
		FOREIGN KEY(conversation_id) REFERENCES conversations(conversation_id) ON DELETE CASCADE
	)`,
	`CREATE INDEX IF NOT EXISTS idx_messages_conversation ON messages(conversation_id, created_at)`,
	`CREATE TABLE IF NOT EXISTS attachments (
		attachment_id TEXT PRIMARY KEY,
		message_id TEXT NOT NULL,
		file_name TEXT NOT NULL,
		mime_type TEXT NOT NULL DEFAULT 'application/octet-stream',
		file_size INTEGER NOT NULL DEFAULT 0,
		sha256 TEXT NOT NULL DEFAULT '',
		thumbnail_data TEXT NOT NULL DEFAULT '',
		thumbnail_mime TEXT NOT NULL DEFAULT '',
		local_path TEXT NOT NULL DEFAULT '',
		status TEXT NOT NULL DEFAULT 'pending',
		created_at TEXT NOT NULL,
		FOREIGN KEY(message_id) REFERENCES messages(message_id) ON DELETE CASCADE
	)`,
	`CREATE INDEX IF NOT EXISTS idx_attachments_message ON attachments(message_id, created_at)`,
	`DROP INDEX IF EXISTS idx_outbox_retry`,
	`DROP TABLE IF EXISTS outbox`,
	`CREATE TABLE IF NOT EXISTS network_diagnostics (
		id TEXT PRIMARY KEY,
		status TEXT NOT NULL,
		result_json TEXT NOT NULL,
		created_at TEXT NOT NULL
	)`,
}
