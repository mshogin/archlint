// App.tsx - React functional and class components.

import React, { useState, useEffect, useCallback } from 'react';
import { UserService } from './sample';
import type { User } from './sample';

interface AppProps {
    title: string;
    userId?: string;
}

// Function component returning JSX.
export function App({ title, userId }: AppProps) {
    const [user, setUser] = useState<User | null>(null);
    const [loading, setLoading] = useState(false);

    useEffect(() => {
        if (!userId) return;
        setLoading(true);
        const svc = new UserService({ info: console.log } as any);
        svc.getUser(userId).then((u) => {
            setUser(u);
            setLoading(false);
        });
    }, [userId]);

    const handleReset = useCallback(() => setUser(null), []);

    if (loading) return <div>Loading…</div>;

    return (
        <div className="app">
            <h1>{title}</h1>
            {user ? <UserCard user={user} onReset={handleReset} /> : <p>No user loaded.</p>}
        </div>
    );
}

interface UserCardProps {
    user: User;
    onReset: () => void;
}

export const UserCard = ({ user, onReset }: UserCardProps) => (
    <div>
        <p>{user.name}</p>
        <p>{user.email}</p>
        <button onClick={onReset}>Reset</button>
    </div>
);

// Class component extending React.Component.
interface ErrorBoundaryState {
    hasError: boolean;
}

export class ErrorBoundary extends React.Component<React.PropsWithChildren<{}>, ErrorBoundaryState> {
    constructor(props: React.PropsWithChildren<{}>) {
        super(props);
        this.state = { hasError: false };
    }

    static getDerivedStateFromError(): ErrorBoundaryState {
        return { hasError: true };
    }

    render() {
        if (this.state.hasError) {
            return <div>Something went wrong.</div>;
        }
        return this.props.children;
    }
}
