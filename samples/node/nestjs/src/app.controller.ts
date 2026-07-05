import { Controller, Get, Header } from '@nestjs/common';

@Controller()
export class AppController {
  @Get()
  @Header('Content-Type', 'text/plain')
  root(): string {
    return 'Hello from nimbopacks — node/nestjs sample\n';
  }

  @Get('healthz')
  healthz(): { status: string } {
    return { status: 'ok' };
  }
}
