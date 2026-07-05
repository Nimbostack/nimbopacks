using MyApp.Worker;

var builder = Host.CreateApplicationBuilder(args);
builder.Services.AddHostedService<Heartbeat>();
builder.Build().Run();
