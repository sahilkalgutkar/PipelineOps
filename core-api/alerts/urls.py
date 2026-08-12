from rest_framework.routers import DefaultRouter

from .views import AlertEventViewSet, AlertRuleViewSet

router = DefaultRouter()
router.register("alert-rules", AlertRuleViewSet, basename="alert-rule")
router.register("alert-events", AlertEventViewSet, basename="alert-event")

urlpatterns = router.urls
