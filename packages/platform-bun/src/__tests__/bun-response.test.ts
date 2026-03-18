import { describe, it, expect } from 'vitest';
import { BunResponse } from '../bun-response';

describe('BunResponse', () => {
  describe('status()', () => {
    it('sets statusCode', () => {
      const res = new BunResponse();
      res.status(201);
      expect(res.statusCode).toBe(201);
    });

    it('returns this for chaining', () => {
      const res = new BunResponse();
      const result = res.status(404);
      expect(result).toBe(res);
    });

    it('defaults to 200', () => {
      const res = new BunResponse();
      expect(res.statusCode).toBe(200);
    });
  });

  describe('getStatus()', () => {
    it('returns the current status code', () => {
      const res = new BunResponse();
      res.status(418);
      expect(res.getStatus()).toBe(418);
    });
  });

  describe('setHeader() / getHeader()', () => {
    it('sets and gets a header', () => {
      const res = new BunResponse();
      res.setHeader('X-Custom', 'value');
      expect(res.getHeader('x-custom')).toBe('value');
    });

    it('normalizes header name to lowercase', () => {
      const res = new BunResponse();
      res.setHeader('Content-Type', 'text/html');
      expect(res.getHeader('content-type')).toBe('text/html');
      expect(res._headers['content-type']).toBe('text/html');
    });

    it('joins array values with comma-space', () => {
      const res = new BunResponse();
      res.setHeader('Set-Cookie', ['a=1', 'b=2']);
      expect(res.getHeader('set-cookie')).toBe('a=1, b=2');
    });

    it('returns undefined for missing header', () => {
      const res = new BunResponse();
      expect(res.getHeader('x-nonexistent')).toBeUndefined();
    });

    it('setHeader returns this for chaining', () => {
      const res = new BunResponse();
      const result = res.setHeader('X-Foo', 'bar');
      expect(result).toBe(res);
    });
  });

  describe('appendHeader()', () => {
    it('appends to existing header with comma separator', () => {
      const res = new BunResponse();
      res.setHeader('X-Multi', 'first');
      res.appendHeader('X-Multi', 'second');
      expect(res.getHeader('x-multi')).toBe('first, second');
    });

    it('creates header if it does not exist', () => {
      const res = new BunResponse();
      res.appendHeader('X-New', 'value');
      expect(res.getHeader('x-new')).toBe('value');
    });

    it('appends multiple times', () => {
      const res = new BunResponse();
      res.appendHeader('Accept', 'text/html');
      res.appendHeader('Accept', 'application/json');
      res.appendHeader('Accept', 'text/plain');
      expect(res.getHeader('accept')).toBe('text/html, application/json, text/plain');
    });

    it('returns this for chaining', () => {
      const res = new BunResponse();
      const result = res.appendHeader('X-Foo', 'bar');
      expect(result).toBe(res);
    });
  });

  describe('removeHeader()', () => {
    it('removes an existing header', () => {
      const res = new BunResponse();
      res.setHeader('X-Remove', 'value');
      res.removeHeader('X-Remove');
      expect(res.getHeader('x-remove')).toBeUndefined();
    });

    it('is a no-op for missing header', () => {
      const res = new BunResponse();
      // Should not throw
      res.removeHeader('X-Nonexistent');
      expect(res.getHeader('x-nonexistent')).toBeUndefined();
    });

    it('returns this for chaining', () => {
      const res = new BunResponse();
      const result = res.removeHeader('X-Foo');
      expect(result).toBe(res);
    });
  });

  describe('json()', () => {
    it('sets content-type to application/json and stringifies body', () => {
      const res = new BunResponse();
      res.json({ message: 'hello' });

      expect(res.getHeader('content-type')).toBe('application/json; charset=utf-8');
      expect(res._body).toBe('{"message":"hello"}');
      expect(res._ended).toBe(true);
    });

    it('handles arrays', () => {
      const res = new BunResponse();
      res.json([1, 2, 3]);

      expect(res._body).toBe('[1,2,3]');
      expect(res._ended).toBe(true);
    });

    it('handles null', () => {
      const res = new BunResponse();
      res.json(null);

      expect(res._body).toBe('null');
    });
  });

  describe('send()', () => {
    it('dispatches to json for objects', () => {
      const res = new BunResponse();
      res.send({ key: 'value' });

      expect(res.getHeader('content-type')).toBe('application/json; charset=utf-8');
      expect(res._body).toBe('{"key":"value"}');
      expect(res._ended).toBe(true);
    });

    it('sends text for strings with text/html content-type', () => {
      const res = new BunResponse();
      res.send('Hello World');

      expect(res.getHeader('content-type')).toBe('text/html; charset=utf-8');
      expect(res._body).toBe('Hello World');
      expect(res._ended).toBe(true);
    });

    it('does not override existing content-type for strings', () => {
      const res = new BunResponse();
      res.setHeader('Content-Type', 'text/plain');
      res.send('Hello World');

      expect(res.getHeader('content-type')).toBe('text/plain');
      expect(res._body).toBe('Hello World');
    });

    it('calls end() for null body', () => {
      const res = new BunResponse();
      res.send(null);

      expect(res._ended).toBe(true);
      expect(res._body).toBeNull();
    });

    it('calls end() for undefined body', () => {
      const res = new BunResponse();
      res.send(undefined);

      expect(res._ended).toBe(true);
    });
  });

  describe('end()', () => {
    it('marks response as ended', () => {
      const res = new BunResponse();
      expect(res._ended).toBe(false);
      res.end();
      expect(res._ended).toBe(true);
    });

    it('double-call is a no-op', () => {
      const res = new BunResponse();
      res.end('first');
      res.end('second');
      expect(res._body).toBe('first');
    });

    it('sets body when provided', () => {
      const res = new BunResponse();
      res.end('done');
      expect(res._body).toBe('done');
    });

    it('does not set body for undefined or null', () => {
      const res = new BunResponse();
      res.end(undefined);
      expect(res._body).toBeNull();

      const res2 = new BunResponse();
      res2.end(null);
      expect(res2._body).toBeNull();
    });
  });

  describe('toResponse()', () => {
    it('builds correct Response with status and headers', () => {
      const res = new BunResponse();
      res.status(201);
      res.setHeader('X-Custom', 'test');
      res.end('Created');

      const response = res.toResponse();
      expect(response.status).toBe(201);
      expect(response.headers.get('x-custom')).toBe('test');
    });

    it('builds Response with string body', async () => {
      const res = new BunResponse();
      res.end('Hello');

      const response = res.toResponse();
      const text = await response.text();
      expect(text).toBe('Hello');
    });

    it('builds Response with null body for 204', async () => {
      const res = new BunResponse();
      res.status(204);
      res.end();

      const response = res.toResponse();
      expect(response.status).toBe(204);
      // body should be null (no content)
      expect(res._body).toBeNull();
    });

    it('builds Response with JSON body', async () => {
      const res = new BunResponse();
      res.json({ ok: true });

      const response = res.toResponse();
      const text = await response.text();
      expect(text).toBe('{"ok":true}');
      expect(response.headers.get('content-type')).toBe('application/json; charset=utf-8');
    });

    it('defaults to status 200', () => {
      const res = new BunResponse();
      res.end();

      const response = res.toResponse();
      expect(response.status).toBe(200);
    });
  });

  describe('getResponse()', () => {
    it('returns resolved promise when already ended', async () => {
      const res = new BunResponse();
      res.status(200);
      res.end('done');

      const response = await res.getResponse();
      expect(response.status).toBe(200);
      const text = await response.text();
      expect(text).toBe('done');
    });

    it('returns pending promise when not yet ended, resolves when end() called later', async () => {
      const res = new BunResponse();
      res.status(200);

      // getResponse() before end()
      const promise = res.getResponse();

      // Simulate deferred end (streaming case)
      setTimeout(() => {
        res.end('streamed');
      }, 10);

      const response = await promise;
      expect(response.status).toBe(200);
      const text = await response.text();
      expect(text).toBe('streamed');
    });

    it('returns the same promise on multiple calls before end()', () => {
      const res = new BunResponse();
      const p1 = res.getResponse();
      const p2 = res.getResponse();
      expect(p1).toBe(p2);
      // Clean up: end the response to avoid dangling promise
      res.end();
    });
  });

  describe('headersSent', () => {
    it('is false before end()', () => {
      const res = new BunResponse();
      expect(res.headersSent).toBe(false);
    });

    it('is true after end()', () => {
      const res = new BunResponse();
      res.end();
      expect(res.headersSent).toBe(true);
    });

    it('is true after json()', () => {
      const res = new BunResponse();
      res.json({ ok: true });
      expect(res.headersSent).toBe(true);
    });

    it('is true after send()', () => {
      const res = new BunResponse();
      res.send('text');
      expect(res.headersSent).toBe(true);
    });
  });

  describe('redirect()', () => {
    it('redirects with url only (defaults to 302)', () => {
      const res = new BunResponse();
      res.redirect('https://example.com');

      expect(res.statusCode).toBe(302);
      expect(res.getHeader('location')).toBe('https://example.com');
      expect(res._ended).toBe(true);
    });

    it('redirects with status and url', () => {
      const res = new BunResponse();
      res.redirect(301, '/new-location');

      expect(res.statusCode).toBe(301);
      expect(res.getHeader('location')).toBe('/new-location');
      expect(res._ended).toBe(true);
    });

    it('builds a valid Response after redirect', async () => {
      const res = new BunResponse();
      res.redirect(307, '/temp');

      const response = res.toResponse();
      expect(response.status).toBe(307);
      expect(response.headers.get('location')).toBe('/temp');
    });
  });

  describe('header() (Express compat)', () => {
    it('acts as setter when value is provided', () => {
      const res = new BunResponse();
      const result = res.header('X-Custom', 'value');
      expect(result).toBe(res); // returns this
      expect(res.getHeader('x-custom')).toBe('value');
    });

    it('acts as getter when no value is provided', () => {
      const res = new BunResponse();
      res.setHeader('X-Custom', 'value');
      const result = res.header('X-Custom');
      expect(result).toBe('value');
    });

    it('returns undefined when getting a missing header', () => {
      const res = new BunResponse();
      const result = res.header('X-Missing');
      expect(result).toBeUndefined();
    });
  });

  describe('type()', () => {
    it('sets content-type header', () => {
      const res = new BunResponse();
      res.type('text/plain');
      expect(res.getHeader('content-type')).toBe('text/plain');
    });

    it('returns this for chaining', () => {
      const res = new BunResponse();
      const result = res.type('application/xml');
      expect(result).toBe(res);
    });
  });
});
