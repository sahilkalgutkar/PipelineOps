from rest_framework.routers import DefaultRouter

from .views import HeartbeatViewSet, JobViewSet

router = DefaultRouter()
router.register("jobs", JobViewSet, basename="job")
router.register("heartbeats", HeartbeatViewSet, basename="heartbeat")

urlpatterns = router.urls
