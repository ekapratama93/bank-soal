from postgrest.exceptions import APIError

from app.db_errors import is_fk_violation, is_unique_violation


def test_is_unique_violation_true_for_23505():
    exc = APIError({"code": "23505", "message": "duplicate key"})
    assert is_unique_violation(exc) is True
    assert is_fk_violation(exc) is False


def test_is_fk_violation_true_for_23503():
    exc = APIError({"code": "23503", "message": "foreign key violation"})
    assert is_fk_violation(exc) is True
    assert is_unique_violation(exc) is False


def test_unrelated_api_error_matches_neither():
    exc = APIError({"code": "42501", "message": "insufficient privilege"})
    assert is_unique_violation(exc) is False
    assert is_fk_violation(exc) is False


def test_non_api_error_matches_neither():
    exc = ValueError("bukan APIError")
    assert is_unique_violation(exc) is False
    assert is_fk_violation(exc) is False
