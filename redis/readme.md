## Решение

### 1. Описание архитектуры

#### Основные компоненты системы:

1. **Очередь задач с приоритетами**:
    - Используем Redis **Sorted Set** для хранения задач с приоритетами.
    - Каждой задаче присваивается числовой приоритет (например, HIGH=3, MEDIUM=2, LOW=1).
    - Sorted Set позволяет эффективно извлекать задачи с наивысшим приоритетом.
2. **Отложенные задачи**:
    - Хранятся в отдельном Sorted Set, где score — это Unix timestamp, когда задача должна быть выполнена.
    - Воркер периодически проверяет задачи, чьё время выполнения наступило, и переносит их в очередь с приоритетами.
3. **Обработчик задач (воркер)**:
    - Реализован как Go-рутина, которая забирает задачи из очереди с приоритетами.
    - Использует механизм блокировки (BRPOPLPUSH) для минимизации опроса Redis.
    - Для отложенных задач использует ZRangeByScore с таймером для проверки.
4. **Гарантии выполнения**:
    - Реализуем механизм подтверждения (acknowledgment) с помощью временной очереди обработки.
    - Если воркер умирает, задача возвращается в очередь с помощью механизма таймаута.
5. **Мониторинг и метрики**:
    - Храним счётчики успешных/проваленных задач в Redis (Hash или Counters).
    - Логируем ошибки с использованием стандартного пакета log или внешнего сервиса (например, Sentry).
6. **Отказоустойчивость**:
    - Используем Redis с репликацией (master-slave) для защиты от сбоев.
    - Для критически важных задач добавляем резервное хранилище (например, запись в файл или другую БД).

#### Выбор структур данных в Redis:

- **Sorted Set** для очереди с приоритетами (priority_queue):
    - Ключ: priority_queue.
    - Score: приоритет задачи (например, 3 для HIGH).
    - Value: JSON-сериализованная задача с уникальным ID.
- **Sorted Set** для отложенных задач (delayed_queue):
    - Ключ: delayed_queue.
    - Score: Unix timestamp выполнения.
    - Value: JSON-сериализованная задача.
- **List** для временной очереди обработки (processing_queue):
    - Используется для отслеживания задач, которые воркер взял в работу.
- **Hash** для метрик (metrics):
    - Хранит счётчики: total_processed, success, failed.

#### Почему именно эти структуры?

- **Sorted Set** идеально подходит для приоритетной очереди, так как позволяет сортировать по score и эффективно извлекать максимальный элемент (ZPOPMAX).
- Для отложенных задач Sorted Set позволяет хранить задачи с точным временем выполнения и извлекать их с помощью ZRangeByScore.
- **List** для обработки задач обеспечивает FIFO-поведение и атомарное извлечение (BRPOPLPUSH).
- **Hash** удобен для хранения и обновления метрик.

### 2. Обоснование выбора решений

#### 2.1. Почему Sorted Set для приоритетной очереди?

- Sorted Set позволяет хранить задачи с приоритетом как score, а метод BZPopMax атомарно извлекает задачу с максимальным приоритетом.
- Это эффективнее, чем использование нескольких List для каждого уровня приоритета, так как не требует опроса нескольких ключей.

#### 2.2. Почему Sorted Set для отложенных задач?

- Score в Sorted Set — это время выполнения задачи, что позволяет использовать ZRangeByScore для извлечения задач, готовых к выполнению.
- Это избавляет от необходимости хранить задачи в List и сканировать их вручную.

#### 2.3. Гарантии выполнения

- **Избежание потери задач**:
    - Задачи, взятые в обработку, помещаются в processing_queue. Если воркер умирает, задачи остаются в этой очереди и могут быть возвращены в priority_queue через механизм таймаута (например, с помощью TTL или периодической проверки).
- **Защита от двойного выполнения**:
    - Используем BRPOPLPUSH (в данном случае аналог через BZPopMax и LPush) для атомарного переноса задачи в processing_queue.
- **Подтверждение выполнения**:
    - Задача удаляется из processing_queue только после успешной обработки, что обеспечивает гарантию выполнения.

#### 2.4. Мониторинг и метрики

- Метрики хранятся в Redis Hash (metrics), что позволяет легко инкрементировать счётчики (HIncrBy) и получать их (HGetAll).
- Логирование ошибок реализовано через log, но в продакшене можно интегрировать с Sentry или ELK.

#### 2.5. Отказоустойчивость

- **Перезапуск Redis**:
    - Используем репликацию Redis (master-slave) и Sentinel для автоматического failover.
    - Для критически важных задач можно настроить периодическую синхронизацию с другой БД (например, PostgreSQL).
- **Резервное хранилище**:
    - При добавлении задачи можно записывать её в файл или другую БД с помощью асинхронного воркера.

### 3. Оптимизации

#### 3.1. Lua-скрипты

Для атомарного извлечения и переноса задач можно использовать Lua-скрипты. Например:
```lua
-- Атомарно извлечь задачу из priority_queue и перенести в processing_queue
local task = redis.call('ZPOPMAX', KEYS[1])
if task[1] then
    redis.call('LPUSH', KEYS[2], task[1])
    return task
end
return nil
```
Это уменьшит количество сетевых запросов.

#### 3.2. Sharding

Если задач слишком много:

- Разделяем priority_queue на несколько ключей (например, priority_queue:shard1, priority_queue:shard2) по хэшу задачи.
- Воркеры распределяются по шардам, что снижает конкуренцию за доступ к Redis.

#### 3.3. Retry при сбоях

Добавляем поле retries в структуру Task и повторяем задачу до N раз при сбоях:
```go
if err := tq.processTask(task); err != nil {
    task.Retries++
    if task.Retries < maxRetries {
        tq.client.ZAdd(tq.ctx, tq.priorityQueueKey, redis.Z{
            Score:  float64(task.Priority),
            Member: taskJSON,
        })
    }
}
```

### 4. Ответы на дополнительные вопросы

#### 4.1. Многопоточная обработка задач

- Запускаем несколько Go-рутин (воркеров), каждая из которых вызывает ProcessTasks.
- Для балансировки нагрузки можно использовать Redis Cluster или шардирование.

#### 4.2. Планировщик для периодических задач

- Добавляем поле Cron в структуру Task, которое определяет периодичность (например, "*/5 * * * *" для выполнения каждые 5 секунд).
- Создаём отдельный Sorted Set (cron_queue) для хранения периодических задач.
- После выполнения задачи проверяем её Cron и, если нужно, добавляем новую задачу с обновлённым ExecuteAt.

#### 4.3. Redis Streams вместо Sorted Set или List

- **Преимущества Redis Streams**:
    - Поддержка consumer groups для распределённой обработки.
    - Встроенный механизм подтверждения (ACK).
    - Хранение истории событий.
- **Как использовать**:
    - Создаём stream tasks_stream для всех задач.
    - Воркеры читают задачи через XREADGROUP, распределяя нагрузку между consumer groups.
    - Для приоритетов добавляем поле priority в сообщение и фильтруем на стороне воркера.
- **Недостатки**:
    - Нет встроенной сортировки по приоритету, как в Sorted Set.
    - Для отложенных задач всё равно нужен отдельный Sorted Set для хранения времени выполнения.
- **Когда использовать**:
    - Если важна распределённая обработка и история событий.
    - В текущем сценарии Sorted Set проще и эффективнее для приоритетов.


## Итоги
- Поддерживает приоритеты и отложенные задачи.
- Гарантирует выполнение через механизм подтверждения.
- Предоставляет метрики и логирование.
- Отказоустойчиво за счёт репликации и обработки сбоев.
- Оптимизировано с использованием Sorted Set и атомарных операций.
