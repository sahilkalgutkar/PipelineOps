from django.contrib import admin
from django.http import JsonResponse
from django.urls import include, path

from . import auth_views


def healthz(request):
    return JsonResponse({"status": "ok"})


urlpatterns = [
    path("admin/", admin.site.urls),
    path("healthz", healthz),
    path("api/auth/me/", auth_views.me),
    path("api/auth/login/", auth_views.login_view),
    path("api/auth/logout/", auth_views.logout_view),
    path("api/", include("jobs.urls")),
    path("api/", include("alerts.urls")),
]
