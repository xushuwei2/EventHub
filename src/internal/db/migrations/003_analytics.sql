CREATE TABLE IF NOT EXISTS track_event (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
    event_id VARCHAR(64) NOT NULL,
    project_id BIGINT UNSIGNED NOT NULL,
    occurred_at DATETIME(3) NOT NULL,
    event_name VARCHAR(128) NOT NULL,
    `release` VARCHAR(64) NOT NULL,
    env VARCHAR(16) NOT NULL,
    route VARCHAR(256) NOT NULL DEFAULT '',
    scene VARCHAR(128) NOT NULL DEFAULT '',
    module VARCHAR(256) NOT NULL DEFAULT '',
    language VARCHAR(16) NOT NULL DEFAULT 'unknown',
    runtime VARCHAR(32) NOT NULL DEFAULT '',
    user_id VARCHAR(128) NOT NULL DEFAULT '',
    room_id VARCHAR(128) NOT NULL DEFAULT '',
    session_id VARCHAR(128) NOT NULL DEFAULT '',
    device_platform VARCHAR(32) NOT NULL DEFAULT 'unknown',
    device_model VARCHAR(128) NOT NULL DEFAULT 'unknown',
    os_version VARCHAR(64) NOT NULL DEFAULT 'unknown',
    sdk_version VARCHAR(64) NOT NULL DEFAULT 'unknown',
    network_type VARCHAR(32) NOT NULL DEFAULT 'unknown',
    funnel_key VARCHAR(64) NOT NULL DEFAULT '',
    step_key VARCHAR(64) NOT NULL DEFAULT '',
    extra_json JSON,
    created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    UNIQUE KEY uk_track_event_id (event_id),
    KEY idx_track_project_occurred (project_id, occurred_at),
    KEY idx_track_user_occurred (project_id, user_id, occurred_at),
    KEY idx_track_funnel (project_id, funnel_key, step_key, occurred_at),
    CONSTRAINT fk_track_project FOREIGN KEY (project_id) REFERENCES report_project(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS user_first_active (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
    project_id BIGINT UNSIGNED NOT NULL,
    user_id VARCHAR(128) NOT NULL,
    first_active_date DATE NOT NULL,
    created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    UNIQUE KEY uk_project_user (project_id, user_id),
    KEY idx_project_first_date (project_id, first_active_date),
    CONSTRAINT fk_first_active_project FOREIGN KEY (project_id) REFERENCES report_project(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS user_daily_active (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
    project_id BIGINT UNSIGNED NOT NULL,
    user_id VARCHAR(128) NOT NULL,
    stat_date DATE NOT NULL,
    event_count BIGINT UNSIGNED NOT NULL DEFAULT 1,
    first_event_at DATETIME(3) NOT NULL,
    last_event_at DATETIME(3) NOT NULL,
    UNIQUE KEY uk_project_user_date (project_id, user_id, stat_date),
    KEY idx_project_stat_date (project_id, stat_date),
    CONSTRAINT fk_daily_active_project FOREIGN KEY (project_id) REFERENCES report_project(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS project_daily_stats (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
    project_id BIGINT UNSIGNED NOT NULL,
    stat_date DATE NOT NULL,
    dau BIGINT UNSIGNED NOT NULL DEFAULT 0,
    new_users BIGINT UNSIGNED NOT NULL DEFAULT 0,
    event_count BIGINT UNSIGNED NOT NULL DEFAULT 0,
    UNIQUE KEY uk_project_date (project_id, stat_date),
    CONSTRAINT fk_project_stats_project FOREIGN KEY (project_id) REFERENCES report_project(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS funnel_definition (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
    project_id BIGINT UNSIGNED NOT NULL,
    funnel_key VARCHAR(64) NOT NULL,
    funnel_name VARCHAR(128) NOT NULL,
    window_hours INT UNSIGNED NOT NULL DEFAULT 24,
    status VARCHAR(16) NOT NULL DEFAULT 'active',
    created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
    UNIQUE KEY uk_project_funnel_key (project_id, funnel_key),
    CONSTRAINT fk_funnel_project FOREIGN KEY (project_id) REFERENCES report_project(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS funnel_step (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
    funnel_id BIGINT UNSIGNED NOT NULL,
    step_key VARCHAR(64) NOT NULL,
    step_name VARCHAR(128) NOT NULL,
    step_order INT UNSIGNED NOT NULL,
    created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    UNIQUE KEY uk_funnel_step_key (funnel_id, step_key),
    UNIQUE KEY uk_funnel_step_order (funnel_id, step_order),
    CONSTRAINT fk_step_funnel FOREIGN KEY (funnel_id) REFERENCES funnel_definition(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
