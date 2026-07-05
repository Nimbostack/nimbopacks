from pathlib import Path

BASE_DIR = Path(__file__).resolve().parent.parent

SECRET_KEY = "django-insecure-sample-key-only-replace-in-production"
DEBUG = False
ALLOWED_HOSTS = ["*"]

INSTALLED_APPS = [
    "django.contrib.staticfiles",
]

MIDDLEWARE: list[str] = []

ROOT_URLCONF = "config.urls"

TEMPLATES = [
    {
        "BACKEND": "django.template.backends.django.DjangoTemplates",
        "DIRS": [],
        "APP_DIRS": True,
        "OPTIONS": {},
    },
]

WSGI_APPLICATION = "config.wsgi.application"

STATIC_URL = "/static/"
DEFAULT_AUTO_FIELD = "django.db.models.BigAutoField"
