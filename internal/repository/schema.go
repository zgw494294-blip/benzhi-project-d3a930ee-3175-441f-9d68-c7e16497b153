package repository

import "database/sql"

func migrate(db *sql.DB) error {
	_, err := db.Exec(`CREATE TABLE IF NOT EXISTS trees(tree_id TEXT PRIMARY KEY,species TEXT NOT NULL,location_description TEXT NOT NULL,protected_status INTEGER NOT NULL,baseline_version INTEGER NOT NULL,created_at TEXT NOT NULL,updated_at TEXT NOT NULL);
CREATE TABLE IF NOT EXISTS batches(batch_id TEXT PRIMARY KEY,tree_id TEXT NOT NULL REFERENCES trees(tree_id),collector TEXT NOT NULL,collected_at TEXT NOT NULL,target_tissues TEXT NOT NULL,target_quantity INTEGER NOT NULL,expected_version INTEGER NOT NULL,status TEXT NOT NULL,created_at TEXT NOT NULL);
CREATE TABLE IF NOT EXISTS inspections(sample_id TEXT PRIMARY KEY,batch_id TEXT NOT NULL REFERENCES batches(batch_id),label TEXT NOT NULL,quantity INTEGER NOT NULL,container_condition TEXT NOT NULL,chain_notes TEXT NOT NULL,quality_status TEXT NOT NULL,review_note TEXT NOT NULL,recorded_at TEXT NOT NULL);
CREATE TABLE IF NOT EXISTS resampling_tasks(task_id TEXT PRIMARY KEY,batch_id TEXT NOT NULL REFERENCES batches(batch_id),reason TEXT NOT NULL,required_actions TEXT NOT NULL,assigned_to TEXT NOT NULL,status TEXT NOT NULL,resolved_at TEXT);
CREATE TABLE IF NOT EXISTS freezes(freeze_id TEXT PRIMARY KEY,batch_id TEXT UNIQUE NOT NULL REFERENCES batches(batch_id),evidence_digest TEXT NOT NULL,frozen_by TEXT NOT NULL,frozen_at TEXT NOT NULL,credential_id TEXT UNIQUE NOT NULL);
CREATE TABLE IF NOT EXISTS credentials(credential_id TEXT PRIMARY KEY,batch_id TEXT NOT NULL REFERENCES batches(batch_id),issued_at TEXT NOT NULL,issuer TEXT NOT NULL,payload_digest TEXT NOT NULL,status TEXT NOT NULL);
CREATE TABLE IF NOT EXISTS audit_events(id INTEGER PRIMARY KEY AUTOINCREMENT,aggregate_type TEXT NOT NULL,aggregate_id TEXT NOT NULL,action TEXT NOT NULL,request_key TEXT NOT NULL,occurred_at TEXT NOT NULL,detail TEXT NOT NULL);
CREATE TABLE IF NOT EXISTS idempotency(request_key TEXT PRIMARY KEY,response TEXT NOT NULL);
CREATE TABLE IF NOT EXISTS operation_results(operation TEXT NOT NULL,request_key TEXT NOT NULL,aggregate_id TEXT NOT NULL,response TEXT NOT NULL,error_code TEXT NOT NULL,error_message TEXT NOT NULL,PRIMARY KEY(operation,request_key));
CREATE TABLE IF NOT EXISTS freeze_snapshots(freeze_id TEXT PRIMARY KEY REFERENCES freezes(freeze_id),snapshot TEXT NOT NULL);
CREATE INDEX IF NOT EXISTS idx_tasks_batch ON resampling_tasks(batch_id);
CREATE INDEX IF NOT EXISTS idx_inspections_batch_label ON inspections(batch_id,label);
CREATE INDEX IF NOT EXISTS idx_audit_aggregate_action ON audit_events(aggregate_id,action,occurred_at);`)
	return err
}
