-- Создаём первого администратора
-- Email: admin@example.com
-- Password: admin123
-- ВАЖНО: Сменить пароль после первого входа в production!

INSERT INTO users (email, password_hash, first_name, last_name, role)
VALUES (
    'admin@example.com',
    '$2a$10$T94g4LxX5xDBAz8TxdzVsOM2s6I8e6YRavJSQgI3azgbcrIO5bU2e',
    'System',
    'Administrator',
    'admin'
) ON CONFLICT (email) DO NOTHING;
