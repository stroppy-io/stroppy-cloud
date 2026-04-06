import { useEffect, useState } from "react";
import {
  listMembers,
  addMember,
  updateMemberRole,
  removeMember,
  listUsersAdmin,
} from "@/api/client";
import type { TenantMember } from "@/api/types";
import { useAuth } from "@/hooks/useAuth";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
  Select,
  SelectTrigger,
  SelectValue,
  SelectContent,
  SelectItem,
} from "@/components/ui/select";
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
import { Plus, Trash2 } from "lucide-react";

interface AdminUser {
  id: string;
  username: string;
  is_root: boolean;
  created_at: string;
}

export function TenantMembers() {
  const { user } = useAuth();
  const [members, setMembers] = useState<TenantMember[]>([]);
  const [allUsers, setAllUsers] = useState<AdminUser[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [open, setOpen] = useState(false);
  const [selectedUserId, setSelectedUserId] = useState("");
  const [selectedRole, setSelectedRole] = useState("viewer");
  const [adding, setAdding] = useState(false);

  async function load() {
    try {
      const m = (await listMembers()) || [];
      setMembers(m);
      // Only root users can list all users for adding members.
      if (user?.is_root) {
        try {
          setAllUsers((await listUsersAdmin()) || []);
        } catch {
          // Not root or endpoint unavailable.
        }
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to load members");
    } finally {
      setLoading(false);
    }
  }

  useEffect(() => {
    load();
  }, []);

  async function handleAdd() {
    if (!selectedUserId) return;
    setAdding(true);
    setError("");
    try {
      await addMember(selectedUserId, selectedRole);
      setSelectedUserId("");
      setSelectedRole("viewer");
      setOpen(false);
      await load();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to add member");
    }
    setAdding(false);
  }

  async function handleRoleChange(userId: string, role: string) {
    setError("");
    try {
      await updateMemberRole(userId, role);
      await load();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to update role");
    }
  }

  async function handleRemove(userId: string, username: string) {
    if (!confirm(`Remove "${username}" from this tenant?`)) return;
    try {
      await removeMember(userId);
      await load();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to remove member");
    }
  }

  // Users not already members.
  const availableUsers = allUsers.filter(
    (u) => !members.some((m) => m.user_id === u.id)
  );

  return (
    <div className="p-6 space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-lg font-semibold">Members</h1>
          <p className="text-sm text-muted-foreground">
            Manage tenant members
          </p>
        </div>
        <Dialog open={open} onOpenChange={setOpen}>
          <DialogTrigger asChild>
            <Button size="sm">
              <Plus className="h-3.5 w-3.5" />
              Add Member
            </Button>
          </DialogTrigger>
          <DialogContent>
            <DialogHeader>
              <DialogTitle>Add Member</DialogTitle>
            </DialogHeader>
            <div className="space-y-4 pt-2">
              {availableUsers.length > 0 ? (
                <>
                  <div className="space-y-2">
                    <Label>User</Label>
                    <Select value={selectedUserId} onValueChange={setSelectedUserId}>
                      <SelectTrigger>
                        <SelectValue placeholder="Select user" />
                      </SelectTrigger>
                      <SelectContent>
                        {availableUsers.map((u) => (
                          <SelectItem key={u.id} value={u.id}>
                            {u.username}
                          </SelectItem>
                        ))}
                      </SelectContent>
                    </Select>
                  </div>
                  <div className="space-y-2">
                    <Label>Role</Label>
                    <Select value={selectedRole} onValueChange={setSelectedRole}>
                      <SelectTrigger>
                        <SelectValue />
                      </SelectTrigger>
                      <SelectContent>
                        <SelectItem value="viewer">Viewer</SelectItem>
                        <SelectItem value="operator">Operator</SelectItem>
                        <SelectItem value="owner">Owner</SelectItem>
                      </SelectContent>
                    </Select>
                  </div>
                  <Button onClick={handleAdd} disabled={adding || !selectedUserId}>
                    {adding ? "Adding..." : "Add"}
                  </Button>
                </>
              ) : (
                <div className="space-y-4">
                  <p className="text-sm text-muted-foreground">
                    {user?.is_root
                      ? "All users are already members of this tenant."
                      : "Enter a user ID to add."}
                  </p>
                  {!user?.is_root && (
                    <>
                      <div className="space-y-2">
                        <Label>User ID</Label>
                        <Input
                          value={selectedUserId}
                          onChange={(e) => setSelectedUserId(e.target.value)}
                          placeholder="user-uuid"
                        />
                      </div>
                      <div className="space-y-2">
                        <Label>Role</Label>
                        <Select value={selectedRole} onValueChange={setSelectedRole}>
                          <SelectTrigger>
                            <SelectValue />
                          </SelectTrigger>
                          <SelectContent>
                            <SelectItem value="viewer">Viewer</SelectItem>
                            <SelectItem value="operator">Operator</SelectItem>
                            <SelectItem value="owner">Owner</SelectItem>
                          </SelectContent>
                        </Select>
                      </div>
                      <Button onClick={handleAdd} disabled={adding || !selectedUserId}>
                        {adding ? "Adding..." : "Add"}
                      </Button>
                    </>
                  )}
                </div>
              )}
            </div>
          </DialogContent>
        </Dialog>
      </div>

      {error && (
        <div className="text-sm text-destructive border border-destructive/30 p-3">
          {error}
        </div>
      )}

      {loading ? (
        <div className="text-sm text-muted-foreground">Loading...</div>
      ) : (
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>Username</TableHead>
              <TableHead>Role</TableHead>
              <TableHead>Added</TableHead>
              <TableHead className="w-10" />
            </TableRow>
          </TableHeader>
          <TableBody>
            {members.map((m) => (
              <TableRow key={m.user_id}>
                <TableCell className="font-medium">{m.username}</TableCell>
                <TableCell>
                  <Select
                    value={m.role}
                    onValueChange={(v) => handleRoleChange(m.user_id, v)}
                  >
                    <SelectTrigger className="h-7 w-28 text-xs">
                      <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                      <SelectItem value="viewer">Viewer</SelectItem>
                      <SelectItem value="operator">Operator</SelectItem>
                      <SelectItem value="owner">Owner</SelectItem>
                    </SelectContent>
                  </Select>
                </TableCell>
                <TableCell className="text-xs text-muted-foreground">
                  {new Date(m.created_at).toLocaleDateString()}
                </TableCell>
                <TableCell>
                  <button
                    onClick={() => handleRemove(m.user_id, m.username)}
                    className="text-muted-foreground hover:text-destructive transition-colors"
                    title="Remove member"
                  >
                    <Trash2 className="h-3.5 w-3.5" />
                  </button>
                </TableCell>
              </TableRow>
            ))}
            {members.length === 0 && (
              <TableRow>
                <TableCell colSpan={4} className="text-center text-muted-foreground">
                  No members
                </TableCell>
              </TableRow>
            )}
          </TableBody>
        </Table>
      )}
    </div>
  );
}
