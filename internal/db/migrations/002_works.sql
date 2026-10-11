-- 作品：成品录入、材料 BOM 快照、制作品次（追加数量与用时）。
-- Works: finished pieces, material BOM snapshots, and production batches (append qty / time).

CREATE TABLE work (
    id                 BIGINT         NOT NULL AUTO_INCREMENT,
    user_id            BIGINT         NOT NULL,
    name               VARCHAR(255)   NOT NULL,
    remark             VARCHAR(1024)  NOT NULL DEFAULT '',
    quantity           DOUBLE         NOT NULL DEFAULT 0,
    total_minutes      DOUBLE         NOT NULL DEFAULT 0,
    avg_minutes        DOUBLE         NOT NULL DEFAULT 0,
    -- 创建时材料总成本（按当时均价锁定）
    -- Total material cost locked at creation from then-current average prices.
    material_cost      DECIMAL(16, 2) NOT NULL DEFAULT 0,
    -- 单件材料成本 = material_cost / 首批 quantity
    -- Per-piece material cost = material_cost / first-batch quantity.
    unit_material_cost DECIMAL(16, 2) NOT NULL DEFAULT 0,
    cover_image        VARCHAR(512)   NOT NULL DEFAULT '',
    created_at         BIGINT         NOT NULL,
    deleted_at         BIGINT         NULL,
    PRIMARY KEY (id),
    KEY idx_work_user (user_id, deleted_at),
    CONSTRAINT fk_work_user FOREIGN KEY (user_id) REFERENCES app_user (id) ON DELETE CASCADE
) ENGINE = InnoDB DEFAULT CHARSET = utf8mb4;

CREATE TABLE work_image (
    id         BIGINT       NOT NULL AUTO_INCREMENT,
    work_id    BIGINT       NOT NULL,
    path       VARCHAR(512) NOT NULL,
    sort_order INT          NOT NULL DEFAULT 0,
    is_cover   TINYINT      NOT NULL DEFAULT 0,
    PRIMARY KEY (id),
    KEY idx_work_image (work_id),
    CONSTRAINT fk_work_image FOREIGN KEY (work_id) REFERENCES work (id) ON DELETE CASCADE
) ENGINE = InnoDB DEFAULT CHARSET = utf8mb4;

-- 材料用量快照：物品删除后仍保留名称与计价信息（不建 item 外键）
-- Material usage snapshot: names and prices survive item deletion (no FK on item_id).
CREATE TABLE work_material (
    id             BIGINT         NOT NULL AUTO_INCREMENT,
    work_id        BIGINT         NOT NULL,
    item_id        BIGINT         NOT NULL,
    warehouse_id   BIGINT         NOT NULL,
    warehouse_name VARCHAR(128)   NOT NULL,
    item_name      VARCHAR(255)   NOT NULL,
    unit           VARCHAR(32)    NOT NULL DEFAULT '',
    quantity       DOUBLE         NOT NULL,
    unit_price     DECIMAL(16, 2) NOT NULL,
    line_cost      DECIMAL(16, 2) NOT NULL,
    PRIMARY KEY (id),
    KEY idx_work_material (work_id),
    CONSTRAINT fk_work_material FOREIGN KEY (work_id) REFERENCES work (id) ON DELETE CASCADE
) ENGINE = InnoDB DEFAULT CHARSET = utf8mb4;

CREATE TABLE work_batch (
    id         BIGINT        NOT NULL AUTO_INCREMENT,
    work_id    BIGINT        NOT NULL,
    quantity   DOUBLE        NOT NULL,
    minutes    DOUBLE        NOT NULL,
    note       VARCHAR(1024) NOT NULL DEFAULT '',
    created_at BIGINT        NOT NULL,
    PRIMARY KEY (id),
    KEY idx_work_batch (work_id, created_at),
    CONSTRAINT fk_work_batch FOREIGN KEY (work_id) REFERENCES work (id) ON DELETE CASCADE
) ENGINE = InnoDB DEFAULT CHARSET = utf8mb4;
