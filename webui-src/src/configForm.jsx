import React, { createContext, useContext, useEffect, useMemo, useState } from 'react';
import { api } from './api.js';
import { useToast } from './App.jsx';

const ConfigFormContext = createContext(null);

export const useConfigForm = () => useContext(ConfigFormContext);

export function newProfile(index, profiles = []) {
  const used = new Set(profiles.map(profile => profile.limbo_upstream).filter(Boolean));
  let port = 19133;
  while (used.has(`127.0.0.1:${port}`)) port += 1;
  const suffix = `${Date.now().toString(36)}-${profiles.length + 1}`;
  return {
    id: `profile-${suffix}`,
    name: `配置方案 ${index + 1}`,
    connection_mode: 'proxy',
    upstream: '127.0.0.1:19132',
    limbo_upstream: `127.0.0.1:${port}`,
    limbo_packs_dir: '',
    limbo_auto_pull_packs: true,
    limbo_form_title: 'CubeDock Limbo',
    limbo_form_content: '点击下方按钮进入服务器。',
    limbo_form_button: '进入服务器',
    lobby_protocol_id: 43,
    room_name: '来和我一起玩吧！',
    room_game_version: '1.21.130.0',
    room_password: '',
    max_players: 10,
    friends_only: false,
    auto_accept_friends: true,
    allow_pvp: true,
    retry_seconds: 10,
  };
}

export function duplicateProfile(profile, profiles) {
  const copy = { ...profile };
  delete copy.id;
  const next = newProfile(profiles.length, profiles);
  return { ...next, ...copy, id: next.id, name: `${profile.name} 副本` };
}

// ConfigFormProvider keeps one editable Config shared by the Settings,
// Profiles and Transfer pages, so editing one page cannot overwrite unsaved
// changes from another.
export function ConfigFormProvider({ status, onChanged, children }) {
  const notify = useToast();
  const [form, setForm] = useState(null);
  const [dirty, setDirty] = useState(false);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');
  const signature = useMemo(() => JSON.stringify(status.config || {}), [status.config]);

  useEffect(() => {
    if (!dirty && status.config) {
      setForm(JSON.parse(JSON.stringify(status.config)));
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [signature, dirty]);

  const update = (field, value) => {
    setForm(current => ({ ...current, [field]: value }));
    setDirty(true);
    setError('');
  };

  const updateProfile = (id, field, value) => {
    setForm(current => ({
      ...current,
      profiles: (current.profiles || []).map(profile =>
        profile.id === id ? { ...profile, [field]: value } : profile,
      ),
    }));
    setDirty(true);
    setError('');
  };

  const addProfile = () => {
    const profiles = form?.profiles || [];
    const profile = newProfile(profiles.length, profiles);
    setForm(current => ({ ...current, profiles: [...(current.profiles || []), profile] }));
    setDirty(true);
    setError('');
    return profile;
  };

  const duplicate = id => {
    const profiles = form?.profiles || [];
    const source = profiles.find(profile => profile.id === id);
    if (!source) return null;
    const profile = duplicateProfile(source, profiles);
    setForm(current => ({ ...current, profiles: [...(current.profiles || []), profile] }));
    setDirty(true);
    setError('');
    return profile;
  };

  const removeProfile = id => {
    setForm(current => ({
      ...current,
      profiles: (current.profiles || []).filter(profile => profile.id !== id),
    }));
    setDirty(true);
    setError('');
  };

  const save = async event => {
    event?.preventDefault();
    if (!form) return false;
    setBusy(true);
    setError('');
    try {
      // Debug is toggled live from the console page; keep the server's value.
      await api('config', { config: { ...form, debug: !!status.config?.debug } });
      notify('设置已保存', 'success');
      setDirty(false);
      onChanged?.();
      return true;
    } catch (failure) {
      setError(failure.message || '保存设置失败');
      return false;
    } finally {
      setBusy(false);
    }
  };

  return (
    <ConfigFormContext.Provider
      value={{
        form,
        update,
        updateProfile,
        addProfile,
        duplicateProfile: duplicate,
        removeProfile,
        save,
        busy,
        error,
        dirty,
        discard: () => setDirty(false),
      }}
    >
      {children}
    </ConfigFormContext.Provider>
  );
}
