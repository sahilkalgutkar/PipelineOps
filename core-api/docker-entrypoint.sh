#!/bin/sh
set -e

echo "Waiting for database..."
python - <<'PYEOF'
import os
import sys
import time

import django

os.environ.setdefault("DJANGO_SETTINGS_MODULE", "pipelineops.settings")
django.setup()

from django.db import connections
from django.db.utils import OperationalError

for _ in range(30):
    try:
        connections["default"].cursor()
        break
    except OperationalError:
        time.sleep(1)
else:
    sys.exit("Database never became available")
PYEOF

python manage.py migrate --noinput

if [ "$DJANGO_CREATE_SUPERUSER" = "true" ]; then
    python manage.py createsuperuser --noinput || true
fi

exec "$@"
