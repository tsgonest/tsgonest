import { describe, it, expect } from 'vitest';
import { BunRequest } from '../bun-request';

describe('BunRequest', () => {
  // Helper to create a Request with the given URL and options
  function makeRequest(url: string, init?: RequestInit): Request {
    return new Request(url, init);
  }

  describe('URL parsing via string slicing', () => {
    it('parses url, originalUrl, and hostname for URL with query string', () => {
      const req = new BunRequest(
        makeRequest('http://localhost:3000/users?page=1&limit=10', {
          headers: { host: 'localhost:3000' },
        }),
      );

      expect(req.url).toBe('/users');
      expect(req.originalUrl).toBe('/users?page=1&limit=10');
      expect(req.hostname).toBe('localhost');
    });

    it('parses url without query string', () => {
      const req = new BunRequest(
        makeRequest('http://localhost:3000/users'),
      );

      expect(req.url).toBe('/users');
      expect(req.originalUrl).toBe('/users');
    });

    it('parses root path correctly', () => {
      const req = new BunRequest(
        makeRequest('http://localhost:3000/'),
      );

      expect(req.url).toBe('/');
      expect(req.originalUrl).toBe('/');
    });

    it('parses deeply nested path', () => {
      const req = new BunRequest(
        makeRequest('http://localhost:3000/api/v2/users/123/posts?sort=desc'),
      );

      expect(req.url).toBe('/api/v2/users/123/posts');
      expect(req.originalUrl).toBe('/api/v2/users/123/posts?sort=desc');
    });

    it('resolves hostname from Host header (strips port)', () => {
      // In Bun.serve(), the Host header is always set by the server.
      // Node.js Request doesn't auto-set it, so we provide it explicitly.
      const req = new BunRequest(
        makeRequest('http://example.com:8080/test', {
          headers: { host: 'example.com:8080' },
        }),
      );

      expect(req.hostname).toBe('example.com');
    });

    it('returns empty hostname when Host header is missing', () => {
      // When no Host header is present, hostname falls back to empty string
      const req = new BunRequest(
        makeRequest('http://myhost:9999/path'),
      );
      expect(req.hostname).toBe('');
    });

    it('resolves hostname without port from Host header', () => {
      const req = new BunRequest(
        makeRequest('http://myhost:9999/path', {
          headers: { host: 'myhost:9999' },
        }),
      );
      expect(req.hostname).toBe('myhost');
    });
  });

  describe('lazy headers', () => {
    it('materializes headers on first access', () => {
      const raw = makeRequest('http://localhost:3000/test', {
        headers: {
          'Content-Type': 'application/json',
          'Authorization': 'Bearer token123',
        },
      });
      const req = new BunRequest(raw);

      // Access headers
      const headers = req.headers;
      expect(headers['content-type']).toBe('application/json');
      expect(headers['authorization']).toBe('Bearer token123');
    });

    it('returns the same object on subsequent access (cached)', () => {
      const req = new BunRequest(
        makeRequest('http://localhost:3000/test', {
          headers: { 'X-Custom': 'value' },
        }),
      );

      const first = req.headers;
      const second = req.headers;
      expect(first).toBe(second);
    });
  });

  describe('lazy query', () => {
    it('returns parsed query parameters', () => {
      const req = new BunRequest(
        makeRequest('http://localhost:3000/users?page=1&limit=10'),
      );

      expect(req.query).toEqual({ page: '1', limit: '10' });
    });

    it('returns empty object when no query string', () => {
      const req = new BunRequest(
        makeRequest('http://localhost:3000/users'),
      );

      const query = req.query;
      expect(Object.keys(query)).toHaveLength(0);
    });

    it('returns the same object on subsequent access (cached)', () => {
      const req = new BunRequest(
        makeRequest('http://localhost:3000/test?a=1'),
      );

      const first = req.query;
      const second = req.query;
      expect(first).toBe(second);
    });

    it('handles single query param', () => {
      const req = new BunRequest(
        makeRequest('http://localhost:3000/search?q=hello+world'),
      );

      expect(req.query).toEqual({ q: 'hello world' });
    });
  });

  describe('get() / header()', () => {
    it('retrieves header value case-insensitively', () => {
      const req = new BunRequest(
        makeRequest('http://localhost:3000/test', {
          headers: { 'Content-Type': 'text/plain' },
        }),
      );

      expect(req.get('content-type')).toBe('text/plain');
      expect(req.get('Content-Type')).toBe('text/plain');
      expect(req.get('CONTENT-TYPE')).toBe('text/plain');
    });

    it('returns undefined for missing header', () => {
      const req = new BunRequest(
        makeRequest('http://localhost:3000/test'),
      );

      expect(req.get('x-nonexistent')).toBeUndefined();
    });

    it('header() delegates to get()', () => {
      const req = new BunRequest(
        makeRequest('http://localhost:3000/test', {
          headers: { 'X-Token': 'abc' },
        }),
      );

      expect(req.header('X-Token')).toBe('abc');
      expect(req.header('x-token')).toBe('abc');
    });
  });

  describe('method', () => {
    it('returns correct HTTP method for GET', () => {
      const req = new BunRequest(makeRequest('http://localhost:3000/'));
      expect(req.method).toBe('GET');
    });

    it('returns correct HTTP method for POST', () => {
      const req = new BunRequest(
        makeRequest('http://localhost:3000/', { method: 'POST' }),
      );
      expect(req.method).toBe('POST');
    });

    it('returns correct HTTP method for DELETE', () => {
      const req = new BunRequest(
        makeRequest('http://localhost:3000/', { method: 'DELETE' }),
      );
      expect(req.method).toBe('DELETE');
    });
  });

  describe('body', () => {
    it('starts as undefined', () => {
      const req = new BunRequest(makeRequest('http://localhost:3000/'));
      expect(req.body).toBeUndefined();
    });

    it('is mutable', () => {
      const req = new BunRequest(makeRequest('http://localhost:3000/'));
      req.body = { name: 'Alice' };
      expect(req.body).toEqual({ name: 'Alice' });
    });

    it('can be set to a string', () => {
      const req = new BunRequest(makeRequest('http://localhost:3000/'));
      req.body = 'raw body text';
      expect(req.body).toBe('raw body text');
    });
  });

  describe('params', () => {
    it('starts as a frozen empty object', () => {
      const req = new BunRequest(makeRequest('http://localhost:3000/'));
      expect(Object.keys(req.params)).toHaveLength(0);
      expect(Object.isFrozen(req.params)).toBe(true);
    });

    it('can be reassigned', () => {
      const req = new BunRequest(makeRequest('http://localhost:3000/'));
      req.params = { id: '42' };
      expect(req.params).toEqual({ id: '42' });
    });
  });

  describe('EventEmitter stubs', () => {
    it('on() returns this', () => {
      const req = new BunRequest(makeRequest('http://localhost:3000/'));
      const result = req.on('data', () => {});
      expect(result).toBe(req);
    });

    it('once() returns this', () => {
      const req = new BunRequest(makeRequest('http://localhost:3000/'));
      const result = req.once('end', () => {});
      expect(result).toBe(req);
    });

    it('off() returns this', () => {
      const req = new BunRequest(makeRequest('http://localhost:3000/'));
      const result = req.off('error', () => {});
      expect(result).toBe(req);
    });

    it('emit() returns false', () => {
      const req = new BunRequest(makeRequest('http://localhost:3000/'));
      const result = req.emit('data', Buffer.from('test'));
      expect(result).toBe(false);
    });

    it('methods are chainable', () => {
      const req = new BunRequest(makeRequest('http://localhost:3000/'));
      const result = req.on('data', () => {}).once('end', () => {}).off('data', () => {});
      expect(result).toBe(req);
    });
  });

  describe('raw', () => {
    it('stores the original Request object', () => {
      const raw = makeRequest('http://localhost:3000/test');
      const req = new BunRequest(raw);
      expect(req.raw).toBe(raw);
    });
  });
});
