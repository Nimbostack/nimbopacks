<?php

namespace App\Controllers;

class Home extends BaseController
{
    public function index(): string
    {
        return '<!DOCTYPE html><html><head><meta charset="utf-8">'
            . '<title>CodeIgniter on Nimbopacks</title></head><body>'
            . '<h1>It works.</h1>'
            . '<p>CodeIgniter ' . \CodeIgniter\CodeIgniter::CI_VERSION
            . ' served from a minimal Wolfi-based image built by nimbopacks.</p>'
            . '</body></html>';
    }
}
