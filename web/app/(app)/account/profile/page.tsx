'use client';

import { useRef, useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';

import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Field } from '@/components/auth/field';
import { useToast } from '@/components/ui/toast';
import { getMe, updateMe, avatarUploadURL, avatarConfirm, putToPresignedUrl, exportMyData } from '@/lib/api/user';

export default function ProfilePage() {
  const queryClient = useQueryClient();
  const { toast } = useToast();
  const fileRef = useRef<HTMLInputElement>(null);

  const { data: me, isLoading } = useQuery({ queryKey: ['me'], queryFn: getMe });
  // Unsaved edit of the name; null shows the saved one (first load, after save).
  const [nameEdit, setName] = useState<string | null>(null);
  const name = nameEdit ?? me?.name ?? '';

  const saveName = useMutation({
    mutationFn: () => updateMe({ name }),
    onSuccess: (p) => {
      queryClient.setQueryData(['me'], p);
      setName(null);
      toast({ title: 'Profile updated', variant: 'success' });
    },
    onError: () => toast({ title: 'Could not update profile', variant: 'error' }),
  });

  const uploadAvatar = useMutation({
    mutationFn: async (file: File) => {
      const { key, url } = await avatarUploadURL({ content_type: file.type, size: file.size });
      await putToPresignedUrl(url, file);
      await avatarConfirm(key);
    },
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: ['me'] });
      toast({ title: 'Avatar updated', variant: 'success' });
    },
    onError: () => toast({ title: 'Avatar upload failed', variant: 'error' }),
  });

  if (isLoading || !me) {
    return <p className="text-sm text-muted-foreground">Loading…</p>;
  }

  return (
    <div className="space-y-6">
      <Card>
        <CardHeader>
          <CardTitle>Profile</CardTitle>
        </CardHeader>
        <CardContent className="space-y-6">
          <div className="flex items-center gap-4">
            {me.avatar_url ? (
              // eslint-disable-next-line @next/next/no-img-element
              <img
                src={me.avatar_url}
                alt="Avatar"
                className="h-16 w-16 rounded-full border object-cover"
              />
            ) : (
              <div className="flex h-16 w-16 items-center justify-center rounded-full border bg-secondary text-lg font-semibold uppercase text-muted-foreground">
                {(me.name || me.email).slice(0, 2)}
              </div>
            )}
            <div>
              <input
                ref={fileRef}
                type="file"
                accept="image/png,image/jpeg,image/webp"
                className="hidden"
                onChange={(e) => {
                  const f = e.target.files?.[0];
                  if (f) uploadAvatar.mutate(f);
                }}
              />
              <Button
                variant="outline"
                size="sm"
                onClick={() => fileRef.current?.click()}
                disabled={uploadAvatar.isPending}
              >
                {uploadAvatar.isPending ? 'Uploading…' : 'Change avatar'}
              </Button>
              <p className="mt-1 text-xs text-muted-foreground">PNG, JPEG or WebP, up to 5 MB.</p>
            </div>
          </div>

          <form
            className="space-y-4"
            onSubmit={(e) => {
              e.preventDefault();
              saveName.mutate();
            }}
          >
            <Field id="name" label="Display name">
              <Input id="name" value={name} onChange={(e) => setName(e.target.value)} required />
            </Field>
            <Button type="submit" disabled={saveName.isPending || name === me.name}>
              {saveName.isPending ? 'Saving…' : 'Save'}
            </Button>
          </form>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>Email</CardTitle>
        </CardHeader>
        <CardContent className="space-y-3">
          <div className="flex items-center gap-2">
            <Input value={me.email} readOnly disabled />
            {me.email_verified ? (
              <span className="whitespace-nowrap text-xs text-green-600">Verified</span>
            ) : (
              <span className="whitespace-nowrap text-xs text-destructive">Unverified</span>
            )}
          </div>
          {/* Email-change is deferred (ADR-015); control stays disabled until the
              backend change-request/confirm flow lands. */}
          <Button variant="outline" size="sm" disabled title="Coming soon">
            Change email (coming soon)
          </Button>
        </CardContent>
      </Card>

      <DataExportCard />
    </div>
  );
}

// Self-service copy of the caller's personal data (KVKK art. 11, GDPR art. 15/20).
function DataExportCard() {
  const { toast } = useToast();
  const download = useMutation({
    mutationFn: exportMyData,
    onSuccess: (data) => {
      const blob = new Blob([JSON.stringify(data, null, 2)], { type: 'application/json' });
      const url = URL.createObjectURL(blob);
      const a = document.createElement('a');
      a.href = url;
      a.download = `fluxboard-personal-data-${new Date().toISOString().slice(0, 10)}.json`;
      a.click();
      URL.revokeObjectURL(url);
    },
    onError: () => toast({ title: 'Could not prepare your data export', variant: 'error' }),
  });

  return (
    <Card>
      <CardHeader>
        <CardTitle>Your data</CardTitle>
      </CardHeader>
      <CardContent className="space-y-3 text-sm text-muted-foreground">
        <p>
          Download a copy of your account data, sign-in identities, organization memberships,
          and sessions as JSON. Content you created inside an organization is controlled by
          that organization; ask an owner or admin to export it.
        </p>
        <Button variant="outline" size="sm" onClick={() => download.mutate()} disabled={download.isPending}>
          {download.isPending ? 'Preparing…' : 'Download my data'}
        </Button>
      </CardContent>
    </Card>
  );
}
