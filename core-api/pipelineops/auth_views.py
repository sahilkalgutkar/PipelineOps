"""Session-cookie auth endpoints for the SPA.

There is no bearer token anywhere in this flow: /login/ establishes a
Django session (httpOnly cookie), /me/ lets the frontend ask "am I still
logged in" after a page refresh (it can't read the cookie itself), and
/logout/ tears the session down. ensure_csrf_cookie must wrap the whole
view (outside @api_view, not applied to a class method) so the csrftoken
cookie gets set even when /me/ 401s for an anonymous visitor — that 401
response is exactly what primes the cookie the frontend needs before its
first login POST.
"""

from django.contrib.auth import authenticate, login, logout
from django.views.decorators.csrf import ensure_csrf_cookie
from rest_framework import status
from rest_framework.decorators import api_view, permission_classes
from rest_framework.permissions import AllowAny, IsAuthenticated
from rest_framework.response import Response


@ensure_csrf_cookie
@api_view(["GET"])
@permission_classes([IsAuthenticated])
def me(request):
    return Response({"username": request.user.username})


@api_view(["POST"])
@permission_classes([AllowAny])
def login_view(request):
    username = request.data.get("username", "")
    password = request.data.get("password", "")
    user = authenticate(request, username=username, password=password)
    if user is None:
        return Response({"detail": "Invalid credentials."}, status=status.HTTP_401_UNAUTHORIZED)
    login(request, user)
    return Response({"username": user.username})


@api_view(["POST"])
@permission_classes([IsAuthenticated])
def logout_view(request):
    logout(request)
    return Response(status=status.HTTP_204_NO_CONTENT)
