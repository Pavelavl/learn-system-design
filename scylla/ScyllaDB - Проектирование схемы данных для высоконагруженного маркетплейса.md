
schema
```cql
-- Таблица для поиска товаров по категории с сортировкой по популярности
CREATE TABLE products_by_category (
    category_id UUID,
    popularity_score INT,
    product_id UUID,
    product_name TEXT,
    price DECIMAL,
    PRIMARY KEY (category_id, popularity_score, product_id)
) WITH CLUSTERING ORDER BY (popularity_score DESC, product_id ASC);

-- Таблица для получения списка заказов пользователя
CREATE TABLE orders_by_user (
    user_id UUID,
    order_id UUID,
    order_date TIMESTAMP,
    status TEXT,
    total_amount DECIMAL,
    PRIMARY KEY (user_id, order_id)
) WITH CLUSTERING ORDER BY (order_id DESC);

-- Таблица для отслеживания статуса конкретного заказа
CREATE TABLE orders_by_id (
    order_id UUID PRIMARY KEY,
    user_id UUID,
    order_date TIMESTAMP,
    status TEXT,
    total_amount DECIMAL,
    items LIST<FROZEN<MAP<TEXT, TEXT>>>

);

-- Таблица для получения списка товаров продавца
CREATE TABLE products_by_seller (
    seller_id UUID,
    product_id UUID,
    product_name TEXT,
    price DECIMAL,
    category_id UUID,
    PRIMARY KEY (seller_id, product_id)
);

-- Таблица для отображения отзывов о товаре
CREATE TABLE reviews_by_product (
    product_id UUID,
    review_id UUID,
    review_date TIMESTAMP,
    author_id UUID,
    rating INT,
    comment TEXT,
    PRIMARY KEY (product_id, review_date, review_id)
) WITH CLUSTERING ORDER BY (review_date DESC, review_id ASC);
```

queries
```cql
-- 1. Поиск товаров по категории с сортировкой по популярности
SELECT * FROM products_by_category WHERE category_id = ? ORDER BY popularity_score DESC;

-- 2. Получение списка заказов пользователя
SELECT * FROM orders_by_user WHERE user_id = ?;

-- 3. Отслеживание статуса конкретного заказа
SELECT status FROM orders_by_id WHERE order_id = ?;

-- 4. Получение списка товаров продавца
SELECT * FROM products_by_seller WHERE seller_id = ?;

-- 5. Отображение отзывов о товаре
SELECT * FROM reviews_by_product WHERE product_id = ? ORDER BY review_date DESC;
```

- **Поиск товаров по категории с сортировкой по популярности**  
    Таблица products_by_category использует category_id как партиционный ключ для группировки товаров по категориям, а popularity_score и product_id как кластерные ключи для сортировки по популярности.
- **Получение списка заказов пользователя**  
    Таблица orders_by_user группирует заказы по user_id (партиционный ключ) и сортирует их по order_id в убывающем порядке для удобного доступа к последним заказам.
- **Отслеживание статуса конкретного заказа**  
    Таблица orders_by_id обеспечивает быстрый доступ к заказу по order_id (первичный ключ) и хранит полную информацию, включая статус.
- **Получение списка товаров продавца**  
    Таблица products_by_seller группирует товары по seller_id (партиционный ключ) для эффективного извлечения всех товаров конкретного продавца.
- **Отображение отзывов о товаре**  
    Таблица reviews_by_product группирует отзывы по product_id (партиционный ключ) и сортирует их по review_date для отображения в хронологическом порядке.

