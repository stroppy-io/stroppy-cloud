import { useEffect, useState } from "react";
import { listAPITokens, createAPIToken, revokeAPIToken } from "@/api/client";
import type { TenantAPIToken } from "@/api/types";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Badge } from "@/components/ui/badge";
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
import { Plus, Trash2, Copy, Check } from "lucide-react";

export function TenantTokens() {
  const [tokens, setTokens] = useState<TenantAPIToken[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");

  // Create dialog
  const [createOpen, setCreateOpen] = useState(false);
  const [tokenName, setTokenName] = useState("");
  const [tokenRole, setTokenRole] = useState("viewer");
  const [tokenExpiry, setTokenExpiry] = useState("");
  const [creating, setCreating] = useState(false);

  // Show plaintext token dialog
  const [plaintext, setPlaintext] = useState("");
  const [copied, setCopied] = useState(false);

  async function load() {
    try {
      setTokens((await listAPITokens()) || []);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to load tokens");
    } finally {
      setLoading(false);
    }
  }

  useEffect(() => {
    load();
  }, []);

  async function handleCreate() {
    if (!tokenName.trim()) return;
    setCreating(true);
    setError("");
    try {
      const result = await createAPIToken(
        tokenName.trim(),
        tokenRole,
        tokenExpiry || undefined
      );
      setPlaintext(result.plaintext);
      setTokenName("");
      setTokenRole("viewer");
      setTokenExpiry("");
      setCreateOpen(false);
      await load();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to create token");
    }
    setCreating(false);
  }

  async function handleRevoke(id: string, name: string) {
    if (!confirm(`Revoke token "${name}"?`)) return;
    try {
      await revokeAPIToken(id);
      await load();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to revoke token");
    }
  }

  function handleCopy() {
    navigator.clipboard.writeText(plaintext);
    setCopied(true);
    setTimeout(() => setCopied(false), 2000);
  }

  return (
    <div className="p-6 space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-lg font-semibold">API Tokens</h1>
          <p className="text-sm text-muted-foreground">
            Manage API tokens for programmatic access
          </p>
        </div>
        <Dialog open={createOpen} onOpenChange={setCreateOpen}>
          <DialogTrigger asChild>
            <Button size="sm">
              <Plus className="h-3.5 w-3.5" />
              New Token
            </Button>
          </DialogTrigger>
          <DialogContent>
            <DialogHeader>
              <DialogTitle>Create API Token</DialogTitle>
            </DialogHeader>
            <div className="space-y-4 pt-2">
              <div className="space-y-2">
                <Label>Name</Label>
                <Input
                  value={tokenName}
                  onChange={(e) => setTokenName(e.target.value)}
                  placeholder="ci-pipeline"
                  autoFocus
                />
              </div>
              <div className="space-y-2">
                <Label>Role</Label>
                <Select value={tokenRole} onValueChange={setTokenRole}>
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
              <div className="space-y-2">
                <Label>
                  Expires At{" "}
                  <span className="text-muted-foreground font-normal">
                    (optional)
                  </span>
                </Label>
                <Input
                  type="datetime-local"
                  value={tokenExpiry}
                  onChange={(e) => setTokenExpiry(e.target.value)}
                />
              </div>
              <Button
                onClick={handleCreate}
                disabled={creating || !tokenName.trim()}
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

      {/* Plaintext token dialog */}
      <Dialog
        open={!!plaintext}
        onOpenChange={(v) => {
          if (!v) {
            setPlaintext("");
            setCopied(false);
          }
        }}
      >
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Token Created</DialogTitle>
          </DialogHeader>
          <div className="space-y-4 pt-2">
            <p className="text-sm text-muted-foreground">
              Copy this token now. It will not be shown again.
            </p>
            <div className="flex items-center gap-2">
              <code className="flex-1 block bg-[#050505] border border-input p-3 font-mono text-xs break-all select-all">
                {plaintext}
              </code>
              <Button variant="outline" size="sm" onClick={handleCopy}>
                {copied ? (
                  <Check className="h-3.5 w-3.5" />
                ) : (
                  <Copy className="h-3.5 w-3.5" />
                )}
              </Button>
            </div>
          </div>
        </DialogContent>
      </Dialog>

      {loading ? (
        <div className="text-sm text-muted-foreground">Loading...</div>
      ) : (
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>Name</TableHead>
              <TableHead>Role</TableHead>
              <TableHead>Created By</TableHead>
              <TableHead>Expires</TableHead>
              <TableHead>Created</TableHead>
              <TableHead className="w-10" />
            </TableRow>
          </TableHeader>
          <TableBody>
            {tokens.map((t) => (
              <TableRow key={t.id}>
                <TableCell className="font-medium">{t.name}</TableCell>
                <TableCell>
                  <Badge variant="secondary">{t.role}</Badge>
                </TableCell>
                <TableCell className="text-xs text-muted-foreground">
                  {t.created_by}
                </TableCell>
                <TableCell className="text-xs text-muted-foreground">
                  {t.expires_at
                    ? new Date(t.expires_at).toLocaleDateString()
                    : "Never"}
                </TableCell>
                <TableCell className="text-xs text-muted-foreground">
                  {new Date(t.created_at).toLocaleDateString()}
                </TableCell>
                <TableCell>
                  <button
                    onClick={() => handleRevoke(t.id, t.name)}
                    className="text-muted-foreground hover:text-destructive transition-colors"
                    title="Revoke token"
                  >
                    <Trash2 className="h-3.5 w-3.5" />
                  </button>
                </TableCell>
              </TableRow>
            ))}
            {tokens.length === 0 && (
              <TableRow>
                <TableCell colSpan={6} className="text-center text-muted-foreground">
                  No tokens
                </TableCell>
              </TableRow>
            )}
          </TableBody>
        </Table>
      )}
    </div>
  );
}
