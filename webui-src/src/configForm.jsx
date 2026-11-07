import React, { createContext, useContext, useEffect, useMemo, useState } from 'react';
import { api } from './api.js';
import { useToast } from './App.jsx';

const ConfigFormContext = createContext(null);

export const useConfigForm = () => useContext(ConfigFormContext);

// ConfigFormProvider keeps one editable config shared by the Settings and
// Transfer pages, so editing one page cannot overwrite unsaved changes from
// the other.
export function ConfigFormProvider({ status, onChanged, children }) {
  const notify = useToast();
  const [form, setForm] = useState(null);
  const [dirty, setDirty] = useState(false);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');
  const signature = useMemo(() => JSON.stringify(status.config || {}), [status.config]);

  useEffect(() => {
    if (!dirty && status.config) {
      setForm({ ...status.config });
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [signature, dirty]);

  const update = (field, value) => {
    setForm(current => ({ ...current, [field]: value }));
    setDirty(true);
    setError('');
  };

  const save = async event => {
    event?.preventDefault();
    if (!form) return;
    setBusy(true);
    setError('');
    try {
      // Debug is toggled live from the console page; keep the server's value.
      await api('config', { config: { ...form, debug: !!status.config?.debug } });
      notify('设置已保存', 'success');
      setDirty(false);
      onChanged?.();
    } catch (failure) {
      setError(failure.message || '保存设置失败');
    } finally {
      setBusy(false);
    }
  };

  return (
    <ConfigFormContext.Provider value={{ form, update, save, busy, error, dirty }}>
      {children}
    </ConfigFormContext.Provider>
  );
}
