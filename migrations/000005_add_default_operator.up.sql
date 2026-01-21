-- Создаём тестового оператора
-- Email: operator@example.com  
-- Password: admin123 (для упрощения используем тот же пароль что у admin)
-- ВАЖНО: Только для development/testing!

INSERT INTO users (email, password_hash, first_name, last_name, role)
VALUES (
    'operator@example.com',
    '$2a$10$T94g4LxX5xDBAz8TxdzVsOM2s6I8e6YRavJSQgI3azgbcrIO5bU2e',
    'Test',
    'Operator',
    'operator'
) ON CONFLICT (email) DO NOTHING;
