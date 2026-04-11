// hooks.ts - custom React hooks.

import { useState, useEffect, useCallback, useRef } from 'react';
import { require as nodeRequire } from 'module';

export type FetchState<T> = {
    data: T | null;
    loading: boolean;
    error: string | null;
};

// useFetch - generic data fetching hook.
export function useFetch<T>(url: string): FetchState<T> {
    const [data, setData] = useState<T | null>(null);
    const [loading, setLoading] = useState(false);
    const [error, setError] = useState<string | null>(null);
    const abortRef = useRef<AbortController | null>(null);

    useEffect(() => {
        abortRef.current = new AbortController();
        setLoading(true);
        fetch(url, { signal: abortRef.current.signal })
            .then((r) => r.json())
            .then((json: T) => {
                setData(json);
                setLoading(false);
            })
            .catch((err: Error) => {
                if (err.name !== 'AbortError') {
                    setError(err.message);
                    setLoading(false);
                }
            });

        return () => {
            abortRef.current?.abort();
        };
    }, [url]);

    return { data, loading, error };
}

// useToggle - simple boolean toggle hook.
export const useToggle = (initial = false) => {
    const [value, setValue] = useState(initial);
    const toggle = useCallback(() => setValue((v) => !v), []);
    return [value, toggle] as const;
};

// Dynamic import example.
async function loadModule(_name: string) {
    const mod = await import('./modules/utils');
    return mod.default;
}

// CommonJS require (legacy).
function legacyLoad(_name: string) {
    // eslint-disable-next-line @typescript-eslint/no-var-requires
    return require('./legacy/compat');
}

export { loadModule, legacyLoad };
