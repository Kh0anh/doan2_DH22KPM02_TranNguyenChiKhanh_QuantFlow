-- 1. Bảng Users
CREATE TABLE users (
    id UUID DEFAULT gen_random_uuid() PRIMARY KEY,
    username VARCHAR(50) NOT NULL UNIQUE,
    password_hash VARCHAR(255) NOT NULL,
    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
);

-- 2. Bảng API_Keys
CREATE TABLE api_keys (
    id UUID DEFAULT gen_random_uuid() PRIMARY KEY,
    user_id UUID NOT NULL,
    exchange VARCHAR(50) DEFAULT 'Binance',
    api_key VARCHAR(255) NOT NULL,
    secret_key_encrypted VARCHAR(512) NOT NULL,
    status VARCHAR(20) DEFAULT 'Connected',
    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT fk_user_api FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);

-- 3. Bảng Strategies
CREATE TABLE strategies (
    id UUID DEFAULT gen_random_uuid() PRIMARY KEY,
    user_id UUID NOT NULL,
    name VARCHAR(100) NOT NULL,
    logic_json JSONB NOT NULL,
    version INT DEFAULT 1,
    status VARCHAR(20) DEFAULT 'Valid',
    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT fk_user_strategy FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);

-- 4. Bảng Bot_Instances
CREATE TABLE bot_instances (
    id UUID DEFAULT gen_random_uuid() PRIMARY KEY,
    user_id UUID NOT NULL,
    strategy_id UUID NOT NULL,
    strategy_version INT NOT NULL,
    bot_name VARCHAR(100) NOT NULL,
    symbol VARCHAR(20) NOT NULL,
    status VARCHAR(20) NOT NULL,
    total_pnl DECIMAL(18,8) DEFAULT 0.0,
    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT fk_user_bot FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
    CONSTRAINT fk_strategy_bot FOREIGN KEY (strategy_id) REFERENCES strategies(id) ON DELETE RESTRICT
);

-- 5. Bảng Bot_Lifecycle_Variables
CREATE TABLE bot_lifecycle_variables (
    id UUID DEFAULT gen_random_uuid() PRIMARY KEY,
    bot_id UUID NOT NULL,
    variable_name VARCHAR(100) NOT NULL,
    variable_value JSONB NOT NULL,
    updated_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT fk_bot_variables FOREIGN KEY (bot_id) REFERENCES bot_instances(id) ON DELETE CASCADE
);

-- 6. Bảng Bot_Logs
CREATE TABLE bot_logs (
    id BIGSERIAL PRIMARY KEY,
    bot_id UUID NOT NULL,
    action_decision VARCHAR(50),
    unit_used INT DEFAULT 0,
    message TEXT,
    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT fk_bot_logs FOREIGN KEY (bot_id) REFERENCES bot_instances(id) ON DELETE CASCADE
);

-- 7. Bảng Trade_History
CREATE TABLE trade_history (
    id UUID DEFAULT gen_random_uuid() PRIMARY KEY,
    user_id UUID NOT NULL,
    bot_id UUID NOT NULL,
    symbol VARCHAR(20) NOT NULL,
    side VARCHAR(10) NOT NULL,
    quantity DECIMAL(18,8) NOT NULL,
    fill_price DECIMAL(18,8) NOT NULL,
    fee DECIMAL(18,8) NOT NULL,
    realized_pnl DECIMAL(18,8) NOT NULL,
    status VARCHAR(20) NOT NULL,
    executed_at TIMESTAMPTZ NOT NULL,
    CONSTRAINT fk_user_trade FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
    CONSTRAINT fk_bot_trade FOREIGN KEY (bot_id) REFERENCES bot_instances(id) ON DELETE CASCADE
);

-- 8. Bảng Candles_Data
CREATE TABLE candles_data (
    id BIGSERIAL PRIMARY KEY,
    symbol VARCHAR(20) NOT NULL,
    open_time TIMESTAMPTZ NOT NULL,
    open_price DECIMAL(18,8) NOT NULL,
    high_price DECIMAL(18,8) NOT NULL,
    low_price DECIMAL(18,8) NOT NULL,
    close_price DECIMAL(18,8) NOT NULL,
    volume DECIMAL(18,8) NOT NULL,
    is_closed BOOLEAN DEFAULT TRUE,
    CONSTRAINT uk_symbol_opentime UNIQUE (symbol, open_time)
);

-- 1. Index tối ưu đọc dữ liệu nến cho Backtest & Chart
-- Sắp xếp symbol tăng dần (mặc định) và open_time giảm dần để lấy dữ liệu mới nhất siêu tốc
CREATE INDEX idx_candles_symbol_time ON candles_data (symbol, open_time DESC);

-- 2. Index cho phép lọc lịch sử lệnh siêu tốc (Kết hợp User, Bot, Symbol và sắp xếp thời gian)
CREATE INDEX idx_trade_history_lookup ON trade_history (user_id, bot_id, symbol, executed_at DESC);

-- 3. Index cho Bot Logs để query phân trang hoặc lấy log mới nhất qua API
CREATE INDEX idx_bot_logs_created_at ON bot_logs (bot_id, created_at DESC);

-- 4. Index cho biến vòng đời để tìm kiếm nhanh theo tên biến của một Bot cụ thể
CREATE INDEX idx_bot_variables_lookup ON bot_lifecycle_variables (bot_id, variable_name);

-- 5. Index để tìm nhanh các Bot đang chạy theo trạng thái
CREATE INDEX idx_bot_status ON bot_instances (status);

