import Database from "better-sqlite3";
import path from "path";

const dbPath = path.join(process.cwd(), "uploads.db");
const db = new Database(dbPath);

// Initialize the database
db.exec(`
  CREATE TABLE IF NOT EXISTS files (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL,
    key TEXT NOT NULL UNIQUE,
    size INTEGER NOT NULL,
    content_type TEXT NOT NULL,
    created_at TEXT DEFAULT CURRENT_TIMESTAMP
  )
`);

export interface FileRecord {
  id: number;
  name: string;
  key: string;
  size: number;
  content_type: string;
  created_at: string;
}

export function saveFile(file: Omit<FileRecord, "id" | "created_at">): void {
  const stmt = db.prepare(`
    INSERT OR REPLACE INTO files (name, key, size, content_type)
    VALUES (?, ?, ?, ?)
  `);
  stmt.run(file.name, file.key, file.size, file.content_type);
}

export function getFiles(): FileRecord[] {
  const stmt = db.prepare(`
    SELECT * FROM files ORDER BY created_at DESC
  `);
  return stmt.all() as FileRecord[];
}

export function deleteFile(key: string): void {
  const stmt = db.prepare(`DELETE FROM files WHERE key = ?`);
  stmt.run(key);
}
