namespace DotnetWorkerSample;

public class Worker : BackgroundService
{
    private readonly ILogger<Worker> _logger;
    private static readonly TimeSpan Interval = TimeSpan.FromSeconds(5);

    public Worker(ILogger<Worker> logger)
    {
        _logger = logger;
    }

    protected override async Task ExecuteAsync(CancellationToken stoppingToken)
    {
        while (!stoppingToken.IsCancellationRequested)
        {
            _logger.LogInformation(
                "Heartbeat from nimbopacks dotnet/worker sample at {Time}",
                DateTimeOffset.Now);
            await Task.Delay(Interval, stoppingToken);
        }
    }
}
