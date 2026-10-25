CREATE TABLE IF NOT EXISTS hotels (
    id SERIAL PRIMARY KEY,
    name VARCHAR(100) UNIQUE,
    city VARCHAR(50),
    price_per_night INT,
    features TEXT
);

INSERT INTO hotels (name, city, price_per_night, features) VALUES 
('Radisson Blu', 'Сочи', 15000, 'Завтрак включен, бассейн, 1 линия, бесплатная отмена'),
('Gorki Gorod', 'Сочи', 8000, 'Горы, без питания, трансфер до моря платный, штраф за отмену'),
('Rixos The Palm', 'Дубай', 35000, 'Ультра всё включено, свой пляж, аквапарк'),
('Ibis Al Barsha', 'Дубай', 6000, 'Только завтрак, далеко от пляжа, рядом метро')
ON CONFLICT (name) DO NOTHING;

-- Место для будущих таблиц:
-- CREATE TABLE flights ...
-- INSERT INTO flights ...