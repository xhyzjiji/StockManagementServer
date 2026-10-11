-- 时间字段统一为毫秒时间戳（BIGINT），与 App 导出数据包格式保持一致
-- All time columns are millisecond timestamps (BIGINT), matching the App export format.

CREATE TABLE app_user (
    id            BIGINT       NOT NULL AUTO_INCREMENT,
    username      VARCHAR(64)  NOT NULL,
    password_hash VARCHAR(255) NOT NULL,
    created_at    BIGINT       NOT NULL,
    PRIMARY KEY (id),
    UNIQUE KEY uk_username (username)
) ENGINE = InnoDB DEFAULT CHARSET = utf8mb4;

CREATE TABLE access_token (
    token_hash CHAR(64) NOT NULL,
    user_id    BIGINT   NOT NULL,
    expires_at BIGINT   NOT NULL,
    created_at BIGINT   NOT NULL,
    PRIMARY KEY (token_hash),
    KEY idx_token_user (user_id),
    KEY idx_token_expires (expires_at),
    CONSTRAINT fk_token_user FOREIGN KEY (user_id) REFERENCES app_user (id) ON DELETE CASCADE
) ENGINE = InnoDB DEFAULT CHARSET = utf8mb4;

CREATE TABLE warehouse (
    id         BIGINT        NOT NULL AUTO_INCREMENT,
    user_id    BIGINT        NOT NULL,
    name       VARCHAR(128)  NOT NULL,
    remark     VARCHAR(1024) NOT NULL DEFAULT '',
    created_at BIGINT        NOT NULL,
    deleted_at BIGINT        NULL,
    PRIMARY KEY (id),
    KEY idx_warehouse_user (user_id, deleted_at),
    CONSTRAINT fk_warehouse_user FOREIGN KEY (user_id) REFERENCES app_user (id) ON DELETE CASCADE
) ENGINE = InnoDB DEFAULT CHARSET = utf8mb4;

CREATE TABLE category (
    id           BIGINT       NOT NULL AUTO_INCREMENT,
    warehouse_id BIGINT       NOT NULL,
    name         VARCHAR(128) NOT NULL,
    created_at   BIGINT       NOT NULL,
    PRIMARY KEY (id),
    KEY idx_category_wh (warehouse_id),
    CONSTRAINT fk_category_wh FOREIGN KEY (warehouse_id) REFERENCES warehouse (id) ON DELETE CASCADE
) ENGINE = InnoDB DEFAULT CHARSET = utf8mb4;

CREATE TABLE item (
    id          BIGINT         NOT NULL AUTO_INCREMENT,
    category_id BIGINT         NOT NULL,
    name        VARCHAR(255)   NOT NULL,
    unit        VARCHAR(32)    NOT NULL DEFAULT '',
    quantity    DOUBLE         NOT NULL DEFAULT 0,
    avg_price   DECIMAL(16, 2) NOT NULL DEFAULT 0,
    total_value DECIMAL(16, 2) NOT NULL DEFAULT 0,
    stock_since BIGINT         NULL,
    created_at  BIGINT         NOT NULL,
    deleted_at  BIGINT         NULL,
    PRIMARY KEY (id),
    KEY idx_item_cat (category_id, deleted_at),
    KEY idx_item_name (name),
    CONSTRAINT fk_item_cat FOREIGN KEY (category_id) REFERENCES category (id) ON DELETE CASCADE
) ENGINE = InnoDB DEFAULT CHARSET = utf8mb4;

CREATE TABLE attribute_def (
    id           BIGINT       NOT NULL AUTO_INCREMENT,
    warehouse_id BIGINT       NOT NULL,
    name         VARCHAR(128) NOT NULL,
    type         VARCHAR(16)  NOT NULL,
    options      TEXT         NOT NULL,
    sort_order   INT          NOT NULL DEFAULT 0,
    PRIMARY KEY (id),
    KEY idx_def_wh (warehouse_id),
    CONSTRAINT fk_def_wh FOREIGN KEY (warehouse_id) REFERENCES warehouse (id) ON DELETE CASCADE
) ENGINE = InnoDB DEFAULT CHARSET = utf8mb4;

CREATE TABLE attribute_value (
    attribute_def_id BIGINT NOT NULL,
    item_id          BIGINT NOT NULL,
    value            TEXT   NOT NULL,
    PRIMARY KEY (attribute_def_id, item_id),
    KEY idx_value_item (item_id),
    CONSTRAINT fk_value_def FOREIGN KEY (attribute_def_id) REFERENCES attribute_def (id) ON DELETE CASCADE,
    CONSTRAINT fk_value_item FOREIGN KEY (item_id) REFERENCES item (id) ON DELETE CASCADE
) ENGINE = InnoDB DEFAULT CHARSET = utf8mb4;

-- 流水的 item_id 不建外键：物品被彻底删除后流水仍保留
-- No FK on item_id: records survive permanent item deletion.
CREATE TABLE stock_record (
    id           BIGINT         NOT NULL AUTO_INCREMENT,
    warehouse_id BIGINT         NOT NULL,
    item_id      BIGINT         NOT NULL,
    item_name    VARCHAR(255)   NOT NULL,
    unit         VARCHAR(32)    NOT NULL DEFAULT '',
    type         VARCHAR(8)     NOT NULL,
    quantity     DOUBLE         NOT NULL,
    before_qty   DOUBLE         NOT NULL,
    after_qty    DOUBLE         NOT NULL,
    total_price  DECIMAL(16, 2) NULL,
    note         VARCHAR(1024)  NOT NULL DEFAULT '',
    created_at   BIGINT         NOT NULL,
    PRIMARY KEY (id),
    KEY idx_record_wh (warehouse_id, created_at),
    KEY idx_record_item (item_id, created_at),
    CONSTRAINT fk_record_wh FOREIGN KEY (warehouse_id) REFERENCES warehouse (id) ON DELETE CASCADE
) ENGINE = InnoDB DEFAULT CHARSET = utf8mb4;
