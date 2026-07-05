using DotnetGrpcSample.Services;
using Microsoft.AspNetCore.Server.Kestrel.Core;

var builder = WebApplication.CreateBuilder(args);

// Listen on 8080 using HTTP/2 in plaintext (h2c) so gRPC works without TLS.
builder.WebHost.ConfigureKestrel(options =>
{
    options.ListenAnyIP(8080, listenOptions =>
    {
        listenOptions.Protocols = HttpProtocols.Http2;
    });
});

builder.Services.AddGrpc();

var app = builder.Build();

app.MapGrpcService<GreeterService>();
app.MapGet("/", () => "gRPC server running. Use a gRPC client (e.g. grpcurl) to call greet.Greeter/SayHello.");

app.Run();
