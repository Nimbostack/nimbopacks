var builder = WebApplication.CreateBuilder(args);
var app = builder.Build();

app.MapGet("/", () => "Hello from nimbopacks — dotnet/minimal-api sample");
app.MapGet("/healthz", () => Results.Ok(new { status = "ok" }));

app.Run();
