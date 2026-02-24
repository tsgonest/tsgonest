/**
 * NestJS interceptor that sets Content-Type to application/json for
 * controller methods that return pre-serialized JSON strings.
 *
 * tsgonest's compile-time rewrite wraps controller return values with
 * `stringifyXxx(await expr)`, which returns a JSON string. Without this
 * interceptor, NestJS/Fastify would send the string as text/plain or
 * double-quote it.
 *
 * With this interceptor, the Content-Type header is set to application/json
 * before the response is sent, so Fastify sends the JSON string as-is.
 *
 * Auto-injected by `tsgonest build` on controller classes that have
 * response serialization.
 */
export class TsgonestSerializeInterceptor {
  intercept(context: any, next: any): any {
    const response = context.switchToHttp().getResponse();
    response.header('Content-Type', 'application/json; charset=utf-8');
    return next.handle();
  }
}
