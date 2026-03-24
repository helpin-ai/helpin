# User Profile Avatar Upload Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Allow users to upload, change, and remove their profile avatar from the Profile settings page.

**Architecture:** Replicate the existing workspace logo upload pattern (multipart form → S3 → public URL stored on user record). Add `UploadAvatar`/`DeleteAvatar` to `AuthService` with S3 client injection. Frontend adds camera overlay + remove button to the existing Profile page avatar, using the same UX as workspace logo in GeneralTab.

**Tech Stack:** Go (Chi handler, GORM repository), React (Profile.tsx), S3/MinIO storage, shadcn/ui Avatar components

---

## File Map

| Action | File | Responsibility |
|--------|------|----------------|
| Modify | `server/internal/service/auth.go` | Add `s3Client` field, `UploadAvatar`, `DeleteAvatar` methods |
| Modify | `server/internal/handler/auth.go` | Add `UploadAvatar`, `DeleteAvatar` handlers |
| Modify | `server/internal/router/router.go` | Register `POST/DELETE /api/auth/me/avatar` |
| Modify | `server/cmd/api/main.go` | Pass `s3Client` to `NewAuthService` |
| Modify | `frontend/src/lib/services/authService.ts` | Add `uploadAvatar`, `deleteAvatar` methods |
| Modify | `frontend/src/pages/Profile.tsx` | Add avatar upload UI with camera overlay + remove button |

---

### Task 1: Backend — Add S3 client to AuthService

**Files:**
- Modify: `server/internal/service/auth.go` (struct + constructor, lines 16-31)
- Modify: `server/cmd/api/main.go` (line 447, wiring)

- [ ] **Step 1: Update AuthService struct and constructor**

In `server/internal/service/auth.go`, add the `s3Client` field and update the constructor:

```go
type AuthService struct {
	userRepo         *repository.UserRepository
	organizationRepo *repository.OrganizationRepository
	jwtManager       *auth.JWTManager
	s3Client         *storage.S3Client
	logger           *slog.Logger
}

func NewAuthService(userRepo *repository.UserRepository, organizationRepo *repository.OrganizationRepository, jwtManager *auth.JWTManager, s3Client *storage.S3Client) *AuthService {
	return &AuthService{
		userRepo:         userRepo,
		organizationRepo: organizationRepo,
		jwtManager:       jwtManager,
		s3Client:         s3Client,
		logger:           slog.Default().With("service", "auth"),
	}
}
```

Add import for `"github.com/helpin-ai/helpin/server/internal/storage"`.

- [ ] **Step 2: Wire S3 client in main.go**

In `server/cmd/api/main.go`, change line 447 from:
```go
authService := service.NewAuthService(userRepo, orgRepo, jwtManager)
```
to:
```go
authService := service.NewAuthService(userRepo, orgRepo, jwtManager, s3Client)
```

- [ ] **Step 3: Verify it compiles**

Run: `cd server && go build ./cmd/api`
Expected: SUCCESS (no errors)

- [ ] **Step 4: Commit**

```bash
git add server/internal/service/auth.go server/cmd/api/main.go
git commit -m "refactor: add S3 client to AuthService for avatar uploads"
```

---

### Task 2: Backend — Add UploadAvatar and DeleteAvatar service methods

**Files:**
- Modify: `server/internal/service/auth.go` (add methods after `UpdateProfile`, around line 175)

- [ ] **Step 1: Add UploadAvatar method**

Add after the `UpdateProfile` method in `server/internal/service/auth.go`:

```go
// UploadAvatar uploads a user avatar to S3 and saves the public URL.
func (s *AuthService) UploadAvatar(ctx context.Context, userID string, body io.Reader, size int64, contentType string) (*model.UserProfile, error) {
	if s.s3Client == nil {
		return nil, fmt.Errorf("file storage not configured")
	}

	// Delete old avatar from S3 if it exists.
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("upload avatar: %w", err)
	}
	if user == nil {
		return nil, fmt.Errorf("user not found")
	}
	if user.AvatarURL != nil && *user.AvatarURL != "" {
		oldKey := fmt.Sprintf("users/%s/avatar/%s", userID, filepath.Base(*user.AvatarURL))
		_ = s.s3Client.DeleteObject(ctx, oldKey)
	}

	ext := ".png"
	switch contentType {
	case "image/jpeg":
		ext = ".jpg"
	case "image/webp":
		ext = ".webp"
	case "image/svg+xml":
		ext = ".svg"
	}
	key := fmt.Sprintf("users/%s/avatar/%s%s", userID, uuid.New().String(), ext)

	if err := s.s3Client.PutObject(ctx, key, contentType, size, body, true); err != nil {
		return nil, fmt.Errorf("upload avatar: %w", err)
	}

	avatarURL := s.s3Client.PublicURL(key)
	user, err = s.userRepo.Update(ctx, userID, nil, &avatarURL, nil)
	if err != nil {
		return nil, fmt.Errorf("upload avatar: %w", err)
	}

	s.logger.InfoContext(ctx, "avatar uploaded", "user_id", userID)
	profile := toUserProfile(user)
	return &profile, nil
}
```

Add imports: `"io"`, `"path/filepath"`, and `"github.com/google/uuid"`.

- [ ] **Step 2: Add DeleteAvatar method**

Add after `UploadAvatar`:

```go
// DeleteAvatar removes the user's avatar.
func (s *AuthService) DeleteAvatar(ctx context.Context, userID string) (*model.UserProfile, error) {
	// Delete old avatar from S3 if it exists.
	existing, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("delete avatar: %w", err)
	}
	if existing != nil && existing.AvatarURL != nil && *existing.AvatarURL != "" && s.s3Client != nil {
		oldKey := fmt.Sprintf("users/%s/avatar/%s", userID, filepath.Base(*existing.AvatarURL))
		_ = s.s3Client.DeleteObject(ctx, oldKey)
	}

	emptyURL := ""
	user, err := s.userRepo.Update(ctx, userID, nil, &emptyURL, nil)
	if err != nil {
		return nil, fmt.Errorf("delete avatar: %w", err)
	}

	s.logger.InfoContext(ctx, "avatar deleted", "user_id", userID)
	profile := toUserProfile(user)
	return &profile, nil
}
```

- [ ] **Step 3: Verify it compiles**

Run: `cd server && go build ./cmd/api`
Expected: SUCCESS

- [ ] **Step 4: Commit**

```bash
git add server/internal/service/auth.go
git commit -m "feat: add UploadAvatar and DeleteAvatar to AuthService"
```

---

### Task 3: Backend — Add handler endpoints and routes

**Files:**
- Modify: `server/internal/handler/auth.go` (add handlers after `ChangePassword`, line 102)
- Modify: `server/internal/router/router.go` (add routes near lines 221-223)

- [ ] **Step 1: Add UploadAvatar handler**

In `server/internal/handler/auth.go`, add after `ChangePassword`:

```go
// UploadAvatar handles POST /api/auth/me/avatar.
func (h *AuthHandler) UploadAvatar(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())

	if err := r.ParseMultipartForm(2 << 20); err != nil {
		writeError(w, http.StatusBadRequest, "file too large (max 2MB)")
		return
	}

	file, header, err := r.FormFile("avatar")
	if err != nil {
		writeError(w, http.StatusBadRequest, "missing avatar file")
		return
	}
	defer file.Close()

	contentType := header.Header.Get("Content-Type")
	if contentType != "image/png" && contentType != "image/jpeg" && contentType != "image/webp" && contentType != "image/svg+xml" {
		writeError(w, http.StatusBadRequest, "only PNG, JPEG, WebP, and SVG images are allowed")
		return
	}

	profile, err := h.authService.UploadAvatar(r.Context(), userID, file, header.Size, contentType)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, profile)
}
```

- [ ] **Step 2: Add DeleteAvatar handler**

```go
// DeleteAvatar handles DELETE /api/auth/me/avatar.
func (h *AuthHandler) DeleteAvatar(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())

	profile, err := h.authService.DeleteAvatar(r.Context(), userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, profile)
}
```

- [ ] **Step 3: Register routes**

In `server/internal/router/router.go`, add after the existing `r.Put("/auth/me", ...)` line (around line 222):

```go
r.Post("/auth/me/avatar", h.Auth.UploadAvatar)
r.Delete("/auth/me/avatar", h.Auth.DeleteAvatar)
```

- [ ] **Step 4: Verify it compiles**

Run: `cd server && go build ./cmd/api`
Expected: SUCCESS

- [ ] **Step 5: Commit**

```bash
git add server/internal/handler/auth.go server/internal/router/router.go
git commit -m "feat: add POST/DELETE /api/auth/me/avatar endpoints"
```

---

### Task 4: Frontend — Add avatar upload/delete API methods

**Files:**
- Modify: `frontend/src/lib/services/authService.ts`

- [ ] **Step 1: Add uploadAvatar and deleteAvatar to authService**

In `frontend/src/lib/services/authService.ts`:

First, update the import to include `API_BASE`:
```ts
import { api, API_BASE } from '../api';
```

Then add before the closing `};`:

```ts
uploadAvatar: async (file: File): Promise<{ data: User | null; error: string | null }> => {
  const token = localStorage.getItem('access_token');
  const formData = new FormData();
  formData.append('avatar', file);
  try {
    const res = await fetch(`${API_BASE}/auth/me/avatar`, {
      method: 'POST',
      headers: token ? { Authorization: `Bearer ${token}` } : {},
      body: formData,
    });
    if (!res.ok) {
      const err = await res.json().catch(() => ({ error: res.statusText }));
      return { data: null, error: err.error || res.statusText };
    }
    const data = await res.json();
    return { data, error: null };
  } catch (e) {
    return { data: null, error: e instanceof Error ? e.message : 'Upload failed' };
  }
},
deleteAvatar: () => api.del<User>('/auth/me/avatar'),
```

Note: `api.upload` does not exist. This uses raw `fetch` with `FormData` — the same pattern as `workspacesService.uploadLogo` in `frontend/src/lib/services/workspacesService.ts` lines 19-38. Do NOT set `Content-Type` header — the browser sets it automatically with the multipart boundary.

- [ ] **Step 2: Commit**

```bash
git add frontend/src/lib/services/authService.ts
git commit -m "feat: add uploadAvatar and deleteAvatar to auth service"
```

---

### Task 5: Frontend — Add avatar upload UI to Profile page

**Files:**
- Modify: `frontend/src/pages/Profile.tsx`

Reference: `frontend/src/components/settings/GeneralTab.tsx` lines 81-116 (upload handler), lines 162-198 (avatar UI with camera overlay)

- [ ] **Step 1: Add imports and state**

In `Profile.tsx`, update imports:

```tsx
import { Avatar, AvatarFallback, AvatarImage } from '@/components/ui/avatar';
import { Camera, Loader2, Mail, Trash2 } from 'lucide-react';
```

Add state after existing state declarations:

```tsx
const [uploadingAvatar, setUploadingAvatar] = useState(false);
```

- [ ] **Step 2: Add upload and delete handlers**

Add after `handleSubmit`:

```tsx
const handleAvatarUpload = async (e: React.ChangeEvent<HTMLInputElement>) => {
  const file = e.target.files?.[0];
  if (!file) return;
  if (!file.type.startsWith('image/')) {
    toast.error('Please select an image file');
    return;
  }
  if (file.size > 2 * 1024 * 1024) {
    toast.error('Image must be under 2MB');
    return;
  }
  setUploadingAvatar(true);
  const { data, error } = await authService.uploadAvatar(file);
  setUploadingAvatar(false);
  e.target.value = '';
  if (error || !data) {
    toast.error(error ?? 'Upload failed');
    return;
  }
  toast.success('Avatar updated');
  useAuthStore.setState({ user: data });
};

const handleRemoveAvatar = async () => {
  setUploadingAvatar(true);
  const { data, error } = await authService.deleteAvatar();
  setUploadingAvatar(false);
  if (error) {
    toast.error(error);
  } else {
    toast.success('Avatar removed');
    if (data) useAuthStore.setState({ user: data });
  }
};
```

- [ ] **Step 3: Update the avatar display**

Replace the existing avatar section (lines 72-83):

```tsx
<div className="flex items-center gap-4">
  <div className="relative group">
    <Avatar className="h-16 w-16">
      {user?.avatar_url && <AvatarImage src={user.avatar_url} alt={user.full_name || 'Avatar'} />}
      <AvatarFallback className="text-lg">{initials}</AvatarFallback>
    </Avatar>
    <label className="absolute inset-0 flex items-center justify-center rounded-full bg-black/50 opacity-0 group-hover:opacity-100 transition-opacity cursor-pointer">
      {uploadingAvatar ? (
        <Loader2 className="h-5 w-5 text-white animate-spin" />
      ) : (
        <Camera className="h-5 w-5 text-white" />
      )}
      <input
        type="file"
        accept="image/*"
        className="hidden"
        onChange={handleAvatarUpload}
        disabled={uploadingAvatar}
      />
    </label>
  </div>
  <div>
    <CardTitle>{user?.full_name || 'User'}</CardTitle>
    <CardDescription className="flex items-center gap-1">
      <Mail className="h-3 w-3" />
      {user?.email}
    </CardDescription>
    {user?.avatar_url && (
      <Button
        variant="ghost"
        size="sm"
        className="h-7 mt-1 text-xs text-destructive hover:text-destructive p-0"
        onClick={handleRemoveAvatar}
        disabled={uploadingAvatar}
      >
        <Trash2 className="h-3 w-3 mr-1" />
        Remove photo
      </Button>
    )}
  </div>
</div>
```

Note: the overlay uses `rounded-full` (not `rounded-lg` like workspace logo) since user avatars are circular.

- [ ] **Step 4: Verify frontend builds**

Run: `cd frontend && pnpm build`
Expected: SUCCESS

- [ ] **Step 5: Commit**

```bash
git add frontend/src/pages/Profile.tsx
git commit -m "feat: add avatar upload/remove UI to Profile page"
```

---

### Task 6: Manual verification

- [ ] **Step 1:** Navigate to `/profile` page
- [ ] **Step 2:** Hover over the avatar — camera icon overlay should appear
- [ ] **Step 3:** Click to upload an image — avatar should update
- [ ] **Step 4:** Verify the avatar shows across the app (comments, member lists, etc.) via the `UserAvatar` component
- [ ] **Step 5:** Click "Remove photo" — should revert to initials
- [ ] **Step 6:** Try uploading a file > 2MB — should show error toast
- [ ] **Step 7:** Try uploading a non-image file — should show error toast
