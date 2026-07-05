var builder = WebApplication.CreateBuilder(args);
var app = builder.Build();

app.MapGet("/", () => "MyApp.Api — handles HTTP requests");
app.MapGet("/healthz", () => Results.Ok(new { status = "ok", role = "api" }));

app.Run();
