namespace MyApp.Worker;

// Logs once per interval to demonstrate that the worker process is alive.
public sealed class Heartbeat(ILogger<Heartbeat> logger) : BackgroundService
{
    protected override async Task ExecuteAsync(CancellationToken stoppingToken)
    {
        while (!stoppingToken.IsCancellationRequested)
        {
            logger.LogInformation("MyApp.Worker heartbeat at {Time:O}", DateTimeOffset.UtcNow);
            await Task.Delay(TimeSpan.FromSeconds(10), stoppingToken);
        }
    }
}
