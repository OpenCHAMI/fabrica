<!--
SPDX-FileCopyrightText: 2025 OpenCHAMI Contributors

SPDX-License-Identifier: MIT
-->

---
layout: page
title: Fabrica Blog
permalink: /blog/
---

Welcome to the Fabrica blog. These short posts explain core ideas in simple terms and point you to working examples.

<ul>
{% for post in site.posts %}
  <li>
    <a href="{{ post.url | relative_url }}">{{ post.title }}</a>
    <small> — {{ post.date | date: "%b %d, %Y" }}</small>
    <div>{{ post.excerpt }}</div>
  </li>
{% endfor %}
</ul>
