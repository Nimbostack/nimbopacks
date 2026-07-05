from django.http import JsonResponse
from django.urls import path


def root(_request):
    return JsonResponse({"message": "Hello from nimbopacks — python/django sample"})


def healthz(_request):
    return JsonResponse({"status": "ok"})


urlpatterns = [
    path("", root),
    path("healthz", healthz),
]
