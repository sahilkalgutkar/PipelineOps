from django.contrib import admin
from django.http import JsonResponse
from django.urls import include, path
from rest_framework.authtoken.views import obtain_auth_token


def healthz(request):
    return JsonResponse({"status": "ok"})


urlpatterns = [
    path("admin/", admin.site.urls),
    path("healthz", healthz),
    path("api/auth/token/", obtain_auth_token),
    path("api/", include("jobs.urls")),
    path("api/", include("alerts.urls")),
]
