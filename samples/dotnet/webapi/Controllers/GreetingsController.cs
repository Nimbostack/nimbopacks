using Microsoft.AspNetCore.Mvc;

namespace DotnetWebapiSample.Controllers;

[ApiController]
[Route("[controller]")]
public class GreetingsController : ControllerBase
{
    [HttpGet]
    public IActionResult Get() =>
        Ok(new { message = "Hello from nimbopacks — dotnet/webapi sample" });

    [HttpGet("{name}")]
    public IActionResult Get(string name) =>
        Ok(new { message = $"Hello, {name}!" });
}
