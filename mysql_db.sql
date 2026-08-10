-- MSOffice2Pdf MySQL bootstrap script
-- Matches current GORM models / AutoMigrate schema (utf8mb4)

-- 1. Create database if missing
CREATE DATABASE IF NOT EXISTS `msoffice2pdf`
    CHARACTER SET utf8mb4
    COLLATE utf8mb4_0900_ai_ci;

-- 2. Select database
USE `msoffice2pdf`;

-- 3. Optional: create app user and grant privileges
CREATE USER IF NOT EXISTS 'msoffice2pdf'@'localhost' IDENTIFIED BY 'msoffice2pdf';
GRANT ALL PRIVILEGES ON `msoffice2pdf`.* TO 'msoffice2pdf'@'localhost';
FLUSH PRIVILEGES;

-- 4. Business tables (see internal/domain/models.go)

CREATE TABLE IF NOT EXISTS `user` (
  `id` bigint NOT NULL AUTO_INCREMENT,
  `uid` varchar(64) NOT NULL,
  `pwd_hash` varchar(255) NOT NULL,
  `token` varchar(255) DEFAULT NULL,
  `role` tinyint NOT NULL DEFAULT '0',
  `status` tinyint NOT NULL DEFAULT '0',
  `created_at` datetime(3) DEFAULT NULL,
  `updated_at` datetime(3) DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `idx_user_uid` (`uid`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS `upload` (
  `id` bigint NOT NULL AUTO_INCREMENT,
  `fileid` varchar(64) NOT NULL,
  `user_id` bigint NOT NULL,
  `original_name` varchar(255) NOT NULL,
  `stored_name` varchar(255) NOT NULL,
  `file_path` varchar(512) NOT NULL,
  `file_size` bigint NOT NULL,
  `status` varchar(20) NOT NULL,
  `error_msg` text,
  `retry_count` bigint NOT NULL DEFAULT '0',
  `last_failed_at` datetime(3) DEFAULT NULL,
  `watermark_text` varchar(255) NOT NULL DEFAULT '',
  `request_id` varchar(128) NOT NULL DEFAULT '',
  `created_at` datetime(3) DEFAULT NULL,
  `updated_at` datetime(3) DEFAULT NULL,
  `deleted_at` datetime(3) DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `idx_upload_file_id` (`fileid`),
  KEY `idx_upload_user_id` (`user_id`),
  KEY `idx_upload_deleted_at` (`deleted_at`),
  KEY `idx_upload_request_id` (`request_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS `upload_history` (
  `id` bigint NOT NULL AUTO_INCREMENT,
  `upload_id` bigint NOT NULL,
  `fileid` varchar(64) NOT NULL,
  `user_id` bigint NOT NULL,
  `original_name` varchar(255) DEFAULT NULL,
  `stored_name` varchar(255) DEFAULT NULL,
  `file_size` bigint NOT NULL DEFAULT '0',
  `final_status` varchar(20) NOT NULL,
  `error_code` varchar(64) NOT NULL DEFAULT '',
  `error_msg` text,
  `retry_count` bigint NOT NULL DEFAULT '0',
  `request_id` varchar(128) NOT NULL DEFAULT '',
  `watermark_text` varchar(255) NOT NULL DEFAULT '',
  `archive_dir` varchar(16) NOT NULL,
  `moved_path` varchar(512) DEFAULT NULL,
  `uploaded_at` datetime(3) NOT NULL,
  `finished_at` datetime(3) NOT NULL,
  `created_at` datetime(3) DEFAULT NULL,
  `deleted_at` datetime(3) DEFAULT NULL,
  PRIMARY KEY (`id`),
  KEY `idx_upload_history_upload_id` (`upload_id`),
  KEY `idx_upload_history_file_id` (`fileid`),
  KEY `idx_upload_history_user_id` (`user_id`),
  KEY `idx_upload_history_deleted_at` (`deleted_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS `pdf` (
  `id` bigint NOT NULL AUTO_INCREMENT,
  `fileid` varchar(64) NOT NULL,
  `upload_id` bigint NOT NULL,
  `user_id` bigint NOT NULL,
  `filename` varchar(255) NOT NULL,
  `file_path` varchar(512) NOT NULL,
  `file_size` bigint NOT NULL,
  `status` varchar(20) NOT NULL,
  `warn_code` varchar(64) NOT NULL DEFAULT '',
  `created_at` datetime(3) DEFAULT NULL,
  `updated_at` datetime(3) DEFAULT NULL,
  `expired_at` datetime(3) DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `idx_pdf_file_id` (`fileid`),
  KEY `idx_pdf_upload_id` (`upload_id`),
  KEY `idx_pdf_user_id` (`user_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS `pdflog` (
  `id` bigint NOT NULL AUTO_INCREMENT,
  `pdf_id` bigint NOT NULL,
  `fileid` varchar(64) NOT NULL DEFAULT '',
  `action` varchar(50) NOT NULL,
  `detail` text,
  `ip_address` varchar(45) DEFAULT NULL,
  `user_agent` varchar(255) DEFAULT NULL,
  `created_at` datetime(3) DEFAULT NULL,
  PRIMARY KEY (`id`),
  KEY `idx_pdflog_pdf_id` (`pdf_id`),
  KEY `idx_pdflog_file_id` (`fileid`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

-- Legacy table: migrated into upload_history at startup; still registered in AutoMigrate
CREATE TABLE IF NOT EXISTS `expired_upload` (
  `id` bigint NOT NULL AUTO_INCREMENT,
  `upload_id` bigint NOT NULL,
  `fileid` varchar(64) NOT NULL,
  `user_id` bigint NOT NULL,
  `original_name` varchar(255) DEFAULT NULL,
  `moved_path` varchar(512) DEFAULT NULL,
  `error_code` varchar(64) NOT NULL DEFAULT '',
  `error_msg` text,
  `expired_at` datetime(3) NOT NULL,
  `created_at` datetime(3) DEFAULT NULL,
  PRIMARY KEY (`id`),
  KEY `idx_expired_upload_upload_id` (`upload_id`),
  KEY `idx_expired_upload_user_id` (`user_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

SELECT 'Database and tables created successfully!' AS Message;
