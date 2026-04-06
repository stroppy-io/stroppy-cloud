import { useEffect, useState } from "react";
import {
  listUsersAdmin,
  createUserAdmin,
  deleteUserAdmin,
  resetPasswordAdmin,
} from "@/api/client";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Badge } from "@/components/ui/badge";
import {
  Table,
  TableHeader,
  TableBody,
  TableHead,
  TableRow,
  TableCell,
} from "@/components/ui/table";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from "@/components/ui/dialog";
import { Plus, Trash2, KeyRound } from "lucide-react";

interface AdminUser {
  id: string;
  username: string;
  is_root: boolean;
  created_at: string;
}

export function AdminUsers() {
  const [users, setUsers] = useState<AdminUser[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");

  // Create dialog
  const [createOpen, setCreateOpen] = useState(false);
  const [newUsername, setNewUsername] = useState("");
  const [newPassword, setNewPassword] = useState("");
  const [newIsRoot, setNewIsRoot] = useState(false);
  const [creating, setCreating] = useState(false);

  // Reset password dialog
  const [resetOpen, setResetOpen] = useState(false);
  const [resetUserId, setResetUserId] = useState("");
  const [resetUserName, setResetUserName] = useState("");
  const [resetPwd, setResetPwd] = useState("");
  const [resetting, setResetting] = useState(false);

  async function load() {
    try {
      setUsers((await listUsersAdmin()) || []);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to load users");
    } finally {
      setLoading(false);
    }
  }

  useEffect(() => {
    load();
  }, []);

  async function handleCreate() {
    if (!newUsername.trim() || !newPassword) return;
    setCreating(true);
    setError("");
    try {
      await createUserAdmin(newUsername.trim(), newPassword, newIsRoot);
      setNewUsername("");
      setNewPassword("");
      setNewIsRoot(false);
      setCreateOpen(false);
      await load();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to create user");
    }
    setCreating(false);
  }

  async function handleDelete(id: string, username: string) {
    if (!confirm(`Delete user "${username}"? This cannot be undone.`)) return;
    try {
      await deleteUserAdmin(id);
      await load();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to delete user");
    }
  }

  async function handleReset() {
    if (!resetPwd) return;
    setResetting(true);
    setError("");
    try {
      await resetPasswordAdmin(resetUserId, resetPwd);
      setResetPwd("");
      setResetOpen(false);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to reset password");
    }
    setResetting(false);
  }

  return (
    <div className="p-6 space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-lg font-semibold">Users</h1>
          <p className="text-sm text-muted-foreground">Manage users (root only)</p>
        </div>
        <Dialog open={createOpen} onOpenChange={setCreateOpen}>
          <DialogTrigger asChild>
            <Button size="sm">
              <Plus className="h-3.5 w-3.5" />
              New User
            </Button>
          </DialogTrigger>
          <DialogContent>
            <DialogHeader>
              <DialogTitle>Create User</DialogTitle>
            </DialogHeader>
            <div className="space-y-4 pt-2">
              <div className="space-y-2">
                <Label>Username</Label>
                <Input
                  value={newUsername}
                  onChange={(e) => setNewUsername(e.target.value)}
                  placeholder="johndoe"
                  autoFocus
                />
              </div>
              <div className="space-y-2">
                <Label>Password</Label>
                <Input
                  type="password"
                  value={newPassword}
                  onChange={(e) => setNewPassword(e.target.value)}
                />
              </div>
              <div className="flex items-center gap-2">
                <input
                  type="checkbox"
                  id="is-root"
                  checked={newIsRoot}
                  onChange={(e) => setNewIsRoot(e.target.checked)}
                  className="accent-primary"
                />
                <Label htmlFor="is-root">Root (superadmin)</Label>
              </div>
              <Button
                onClick={handleCreate}
                disabled={creating || !newUsername.trim() || !newPassword}
              >
                {creating ? "Creating..." : "Create"}
              </Button>
            </div>
          </DialogContent>
        </Dialog>
      </div>

      {error && (
        <div className="text-sm text-destructive border border-destructive/30 p-3">
          {error}
        </div>
      )}

      {/* Reset password dialog */}
      <Dialog open={resetOpen} onOpenChange={setResetOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Reset Password for {resetUserName}</DialogTitle>
          </DialogHeader>
          <div className="space-y-4 pt-2">
            <div className="space-y-2">
              <Label>New Password</Label>
              <Input
                type="password"
                value={resetPwd}
                onChange={(e) => setResetPwd(e.target.value)}
                autoFocus
              />
            </div>
            <Button onClick={handleReset} disabled={resetting || !resetPwd}>
              {resetting ? "Resetting..." : "Reset Password"}
            </Button>
          </div>
        </DialogContent>
      </Dialog>

      {loading ? (
        <div className="text-sm text-muted-foreground">Loading...</div>
      ) : (
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>Username</TableHead>
              <TableHead>Role</TableHead>
              <TableHead>Created</TableHead>
              <TableHead className="w-20" />
            </TableRow>
          </TableHeader>
          <TableBody>
            {users.map((u) => (
              <TableRow key={u.id}>
                <TableCell className="font-medium">{u.username}</TableCell>
                <TableCell>
                  {u.is_root ? (
                    <Badge variant="destructive">root</Badge>
                  ) : (
                    <Badge variant="secondary">user</Badge>
                  )}
                </TableCell>
                <TableCell className="text-xs text-muted-foreground">
                  {new Date(u.created_at).toLocaleDateString()}
                </TableCell>
                <TableCell>
                  <div className="flex items-center gap-2">
                    <button
                      onClick={() => {
                        setResetUserId(u.id);
                        setResetUserName(u.username);
                        setResetPwd("");
                        setResetOpen(true);
                      }}
                      className="text-muted-foreground hover:text-foreground transition-colors"
                      title="Reset password"
                    >
                      <KeyRound className="h-3.5 w-3.5" />
                    </button>
                    <button
                      onClick={() => handleDelete(u.id, u.username)}
                      className="text-muted-foreground hover:text-destructive transition-colors"
                      title="Delete user"
                    >
                      <Trash2 className="h-3.5 w-3.5" />
                    </button>
                  </div>
                </TableCell>
              </TableRow>
            ))}
            {users.length === 0 && (
              <TableRow>
                <TableCell colSpan={4} className="text-center text-muted-foreground">
                  No users
                </TableCell>
              </TableRow>
            )}
          </TableBody>
        </Table>
      )}
    </div>
  );
}
