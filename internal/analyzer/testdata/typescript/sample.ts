// sample.ts - plain TypeScript with class, interface, type alias, and functions.

import { EventEmitter } from 'events';
import type { Logger } from '../logger';
import { formatDate } from './utils';

export interface IUserService {
    getUser(id: string): Promise<User>;
    createUser(data: UserInput): Promise<User>;
}

export type UserInput = {
    name: string;
    email: string;
    role?: string;
};

export type UserId = string;

export class UserService implements IUserService {
    private emitter: EventEmitter;

    constructor(private logger: Logger) {
        this.emitter = new EventEmitter();
    }

    async getUser(id: string): Promise<User> {
        this.logger.info(`fetching user ${id}`);
        return { id, name: 'Alice', email: 'alice@example.com', createdAt: formatDate(new Date()) };
    }

    async createUser(data: UserInput): Promise<User> {
        const id = Math.random().toString(36).slice(2);
        this.emitter.emit('user:created', id);
        return { id, name: data.name, email: data.email, createdAt: formatDate(new Date()) };
    }
}

export interface User {
    id: string;
    name: string;
    email: string;
    createdAt: string;
}

export function parseUserInput(raw: unknown): UserInput {
    if (typeof raw !== 'object' || raw === null) {
        throw new Error('Invalid input');
    }
    const obj = raw as Record<string, unknown>;
    return {
        name: String(obj['name'] ?? ''),
        email: String(obj['email'] ?? ''),
        role: obj['role'] !== undefined ? String(obj['role']) : undefined,
    };
}

const sanitizeEmail = (email: string): string => email.trim().toLowerCase();

export { sanitizeEmail };
