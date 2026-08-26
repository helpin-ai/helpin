import hashlib
import logging
import sentry_sdk

import os
from contextlib import contextmanager
from datetime import timedelta
from functools import wraps
from urllib.parse import parse_qs, urlparse

import clickhouse_driver
import pendulum
import psycopg2
from dotenv import load_dotenv
from psycopg2.pool import SimpleConnectionPool

# Load environment variables from .env file
load_dotenv()

# Set up logging
logging.basicConfig(level=logging.INFO, format='%(asctime)s - %(levelname)s - %(message)s')
logger = logging.getLogger(__name__)


sentry_sdk.init(
    dsn=os.getenv("RETROACTIVE_SENTRY_DSN"),
    # Set traces_sample_rate to 1.0 to capture 100%
    # of transactions for performance monitoring.
    traces_sample_rate=1.0,
)

# Database connection details
CH_PARAMS = {
    'host': os.getenv('CLICKHOUSE_CLIENT_HOST'),
    'port': int(os.getenv('CLICKHOUSE_CLIENT_PORT', 9000)),
    'user': os.getenv('CLICKHOUSE_CLIENT_USER'),
    'password': os.getenv('CLICKHOUSE_CLIENT_PASSWORD'),
    'database': os.getenv('CLICKHOUSE_DATABASE'),
}

print(CH_PARAMS)


db_url = os.getenv('POSTGRES_DATABASE_URL', 'postgresql://owner:xxxx@ehost/database?sslmode=require')


url = urlparse(db_url)
query_params = parse_qs(url.query)

# Extract the endpoint ID (first part of the hostname)
endpoint_id = url.hostname.split('.')[0]

PG_PARAMS = {
    'dbname': url.path[1:],
    'user': url.username,
    'password': url.password,
    'host': url.hostname,
    'port': url.port or 5432,
    'sslmode': 'require',
    'options': f'endpoint={endpoint_id}'  # Changed from '-c endpoint={endpoint_id}'
}

# Add any additional query parameters
for key, value in query_params.items():
    if key not in PG_PARAMS:
        PG_PARAMS[key] = value[0]


class DatabasePool:
    def __init__(self, db_type, **db_params):
        if db_type == 'postgres':
            self.pool = SimpleConnectionPool(1, 10, **db_params)
        elif db_type == 'clickhouse':
            self.client = clickhouse_driver.Client(**db_params)
        else:
            raise ValueError(f"Unsupported database type: {db_type}")
        self.db_type = db_type

    @contextmanager
    def get_connection(self):
        if self.db_type == 'postgres':
            conn = self.pool.getconn()
            try:
                yield conn
            finally:
                self.pool.putconn(conn)
        elif self.db_type == 'clickhouse':
            yield self.client

    def close(self):
        if self.db_type == 'postgres':
            self.pool.closeall()

pg_pool = DatabasePool('postgres', **PG_PARAMS)
ch_pool = DatabasePool('clickhouse', **CH_PARAMS)

def error_handler(func):
    @wraps(func)
    def wrapper(*args, **kwargs):
        try:
            return func(*args, **kwargs)
        except Exception as e:
            logger.exception(f"Error in {func.__name__}: {str(e)}")
            raise
    return wrapper

def get_database_time(db_connection):
    if isinstance(db_connection, clickhouse_driver.Client):
        print(db_connection.execute("SELECT now()"))
        return pendulum.instance(db_connection.execute("SELECT now()")[0][0]).in_timezone('UTC')
    else:
        with db_connection.cursor() as cur:
            cur.execute("SELECT current_timestamp AT TIME ZONE 'UTC'")
            return pendulum.instance(cur.fetchone()[0]).in_timezone('UTC')

def generate_run_id(start_time, end_time):
    return hashlib.md5(f"{start_time}-{end_time}".encode()).hexdigest()

@error_handler
def get_last_checkpoint():
    with pg_pool.get_connection() as conn:
        with conn.cursor() as cur:
            cur.execute("SELECT last_timestamp FROM checkpoints ORDER BY id DESC LIMIT 1")
            result = cur.fetchone()
            return pendulum.instance(result[0]) if result else None

@error_handler
def update_checkpoint(timestamp):
    with pg_pool.get_connection() as conn:
        with conn.cursor() as cur:
            cur.execute(
                "INSERT INTO checkpoints (last_timestamp) VALUES (%s)",
                (timestamp,)
            )
        conn.commit()

@error_handler
def run_clickhouse_query(start_timestamp, end_timestamp):
     # Format timestamps for ClickHouse
    start_timestamp_str = start_timestamp.format('YYYY-MM-DD HH:mm:ss')
    end_timestamp_str = end_timestamp.format('YYYY-MM-DD HH:mm:ss')
    query = f"""
    INSERT INTO usermaven.events
    (
        raw_event,
        _nats_subject,
        _nats_stream_sequence,
        _nats_delivery_attempt,
        _retro_generation,
        _ingest_version,
        _written_at
    )
    WITH retro_window_users AS
    (
        SELECT
            user_anonymous_id,
            argMin(user_id, _timestamp) AS user_id,
            project_id
        FROM usermaven.events FINAL
        WHERE ((_timestamp >= toDateTime('{start_timestamp_str}')) AND (_timestamp <= toDateTime('{end_timestamp_str}'))) AND (event_type = 'user_identify')
        GROUP BY
            user_anonymous_id,
            project_id
    )
    SELECT
        jsonMergePatch(
            left.raw_event,
            concat('{"user_id":', toJSONString(right.right_user_id), '}')
        ) AS raw_event,
        left._nats_subject,
        left._nats_stream_sequence,
        left._nats_delivery_attempt,
        toUInt8(left._retro_generation + 1) AS _retro_generation,
        (left._nats_stream_sequence * 65536)
            + ((left._retro_generation + 1) * 256)
            + left._nats_delivery_attempt AS _ingest_version,
        now64(3, 'UTC') AS _written_at
    FROM
    (
        SELECT
            raw_event,
            _nats_subject,
            _nats_stream_sequence,
            _nats_delivery_attempt,
            _retro_generation,
            project_id,
            user_anonymous_id
        FROM usermaven.events FINAL
        WHERE ((_timestamp >= (now() - toIntervalMonth(6))) AND (_timestamp <= now())) AND ((project_id, user_anonymous_id) IN (
            SELECT
                project_id,
                user_anonymous_id
            FROM retro_window_users
        )) AND (user_id = '') AND (_retro_generation < 255)
    ) AS left
    INNER JOIN
    (
        SELECT
            project_id AS right_project_id,
            user_id AS right_user_id,
            user_anonymous_id AS right_user_anonymous_id
        FROM retro_window_users
    ) AS right ON (left.user_anonymous_id = right.right_user_anonymous_id) AND (left.project_id = right.right_project_id)
    """

    with ch_pool.get_connection() as client:
        result = client.execute(query, {
            'start_timestamp': start_timestamp.isoformat(),
            'end_timestamp': end_timestamp.isoformat()
        })
        logger.info(result)
        logger.info(f"Processed {len(result)} rows")
    return result

@error_handler
def process_data_idempotently(start_time, end_time):
    run_id = generate_run_id(start_time.isoformat(), end_time.isoformat())
    
    with pg_pool.get_connection() as conn:
        with conn.cursor() as cur:
            # Check if this run has already been processed
            cur.execute("SELECT 1 FROM processed_runs WHERE run_id = %s", (run_id,))
            if cur.fetchone():
                logger.info(f"Run {run_id} already processed. Skipping.")
                return

            # Process data
            run_clickhouse_query(start_time, end_time)

            # Mark run as processed
            cur.execute("INSERT INTO processed_runs (run_id, start_time, end_time) VALUES (%s, %s, %s)",
                        (run_id, start_time, end_time))
        conn.commit()

@error_handler
def main():
    try:
        with ch_pool.get_connection() as ch_client, pg_pool.get_connection() as pg_conn:
            ch_time = get_database_time(ch_client)
            pg_time = get_database_time(pg_conn)

            if abs((ch_time - pg_time).total_seconds()) > 5:
                raise ValueError(f"Time mismatch between ClickHouse ({ch_time}) and PostgreSQL ({pg_time})")

            end_time = min(ch_time, pg_time)
            start_time = get_last_checkpoint()
            if start_time is None:
                start_time = end_time.subtract(hours=1)

            logger.info(f"Processing data from {start_time} to {end_time}")
            process_data_idempotently(start_time, end_time)
            update_checkpoint(end_time)
            logger.info("Job completed successfully")
    finally:
        pg_pool.close()

if __name__ == "__main__":
    try:
        main()
    except Exception as e:
        logger.error(f"Job failed: {str(e)}")
        raise
