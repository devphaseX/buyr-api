package main

import (
	"net/http"

	"github.com/devphaseX/buyr-api.git/internal/store"
)

type AuthInfo struct {
	*store.User
	AdminUser   *store.AdminUser
	IsAnonymous bool

	// Calculated properties
	IsSuperAdmin   bool
	IsManagerAdmin bool
}

type PermissionCheck func(*AuthInfo) bool

func MinimumAdminLevel(level store.AdminLevel) PermissionCheck {
	return func(a *AuthInfo) bool {
		if a.IsAnonymous || a.User.Role != store.AdminRole {
			return false
		}
		return a.AdminUser.AdminLevel.HasAccessTo(level)
	}
}

func (a *AuthInfo) populateAdminFlags() {
	if a.User != nil && a.User.Role == store.AdminRole {
		a.IsSuperAdmin = a.AdminUser.AdminLevel.HasAccessTo(store.AdminLevelSuper)
		a.IsManagerAdmin = a.AdminUser.AdminLevel.HasAccessTo(store.AdminLevelManager)
	}
}

func (a *AuthInfo) CanManageUsers() bool {
	return a.IsManagerAdmin || a.IsSuperAdmin
}

func (a *AuthInfo) CanModifySystemSettings() bool {
	return a.IsSuperAdmin
}

// Add utility methods to AuthInfo
func (a *AuthInfo) IsRegularUser() bool {
	return a.User.Role == store.UserRole
}

func (a *AuthInfo) IsVendor() bool {
	return a.User.Role == store.VendorRole
}

func (a *AuthInfo) IsUserOrVendor() bool {
	return a.IsRegularUser() || a.IsVendor()
}

func RequireAny(checks ...PermissionCheck) PermissionCheck {
	return func(a *AuthInfo) bool {
		for _, check := range checks {
			if check(a) {
				return true
			}
		}
		return false
	}
}

func All(checks ...PermissionCheck) PermissionCheck {
	return func(a *AuthInfo) bool {
		for _, check := range checks {
			if !check(a) {
				return false
			}
		}
		return true
	}
}

func RequireRoles(allowedRoles ...store.Role) PermissionCheck {
	return func(a *AuthInfo) bool {
		if a.IsAnonymous {
			return false
		}
		for _, allowed := range allowedRoles {
			if a.User.Role == allowed {
				return true
			}
		}
		return false
	}
}

func (app *application) CheckPermissions(checks ...PermissionCheck) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authInfo := getUserFromCtx(r)
			if authInfo.IsAnonymous {
				app.unauthorizedResponse(w, r, "you are required to sign in")
				return
			}

			for _, check := range checks {
				if !check(authInfo) {
					app.forbiddenResponse(w, r)
					return
				}
			}

			next.ServeHTTP(w, r)
		})
	}
}
